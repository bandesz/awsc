package autoscaling

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// MigrateInstances replaces all the instances in an auto scaling group one-by-one
func MigrateInstances(config *aws.Config, out io.Writer, name string, ecsCluster string) error {
	sess := session.Must(session.NewSession(config))
	service := autoscaling.New(sess)
	ecsService := ecs.New(sess)

	ecsClusterInstances := map[string]string{}
	var err error
	if ecsCluster != "" {
		ecsClusterInstances, err = getECSClusterInstances(ecsService, ecsCluster)
		if err != nil {
			return fmt.Errorf("failed to get ECS container instances for %s: %s", ecsCluster, err)
		}
	}

	group, err := getAutoScalingGroup(service, name)
	if err != nil {
		return err
	}

	oldInstances := group.Instances
	instanceCount := len(oldInstances)
	fmt.Fprintf(out, "Instance count: %d\n", instanceCount)

	for _, oldInstance := range oldInstances {
		if ecsInstance, isMember := ecsClusterInstances[*oldInstance.InstanceId]; isMember {
			drainECSInstance(ecsService, *oldInstance.InstanceId, ecsInstance, ecsCluster, out)
		}

		fmt.Fprintf(out, "Terminating %s\n", *oldInstance.InstanceId)
		_, err := service.TerminateInstanceInAutoScalingGroup(
			&autoscaling.TerminateInstanceInAutoScalingGroupInput{
				InstanceId:                     oldInstance.InstanceId,
				ShouldDecrementDesiredCapacity: aws.Bool(false),
			},
		)
		if err != nil {
			return fmt.Errorf("failed to terminate instance: %s", err.Error())
		}
		for {
			time.Sleep(10 * time.Second)

			group, err := getAutoScalingGroup(service, name)
			if err != nil {
				fmt.Fprintf(out, "Failed to get auto scaling group, retrying..")
				time.Sleep(3 * time.Second)
			}

			stillTerminating := false
			hasOutOfServiceInstance := false

			if len(group.Instances) < instanceCount {
				hasOutOfServiceInstance = true
				fmt.Fprintln(out, "Waiting for a new instance to be created..")
			} else {
				for _, newInstance := range group.Instances {
					if *newInstance.InstanceId == *oldInstance.InstanceId {
						stillTerminating = true
						fmt.Fprintf(out, "Waiting for %s to be terminated..\n", *oldInstance.InstanceId)
						break
					}
					if *newInstance.LifecycleState != autoscaling.LifecycleStateInService ||
						*newInstance.HealthStatus != "Healthy" {
						hasOutOfServiceInstance = true
						fmt.Fprintf(out, "Waiting for %s to be in service..\n", *newInstance.InstanceId)
					}
				}
			}

			if !stillTerminating && !hasOutOfServiceInstance {
				break
			}
		}
	}
	fmt.Fprintln(out, "Done.")
	return nil
}

func getAutoScalingGroup(service *autoscaling.AutoScaling, name string) (*autoscaling.Group, error) {
	output, err := service.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{name}),
	})
	if err != nil {
		return nil, err
	}
	if len(output.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("auto scaling group does not exist: %s", name)
	}
	return output.AutoScalingGroups[0], nil
}

func getECSClusterInstances(service *ecs.ECS, clusterName string) (map[string]string, error) {
	instanceARNs := []*string{}
	err := service.ListContainerInstancesPages(
		&ecs.ListContainerInstancesInput{Cluster: aws.String(clusterName)},
		func(page *ecs.ListContainerInstancesOutput, _ bool) bool {
			instanceARNs = append(instanceARNs, page.ContainerInstanceArns...)
			return true
		},
	)
	if err != nil {
		return nil, err
	}

	if len(instanceARNs) == 0 {
		return map[string]string{}, nil
	}

	res := map[string]string{}
	for i := 0; i < len(instanceARNs); i += 100 {
		instances, err := service.DescribeContainerInstances(&ecs.DescribeContainerInstancesInput{
			Cluster:            aws.String(clusterName),
			ContainerInstances: instanceARNs[i:min(i+100, len(instanceARNs))],
		})
		if err != nil {
			return nil, err
		}

		for _, instance := range instances.ContainerInstances {
			res[*instance.Ec2InstanceId] = *instance.ContainerInstanceArn
		}
	}
	return res, nil
}

func drainECSInstance(service *ecs.ECS, ec2InstanceID string, instanceARN string, clusterName string, out io.Writer) error {
	fmt.Fprintf(out, "Draining %s in ECS cluster %s\n", ec2InstanceID, clusterName)
	_, err := service.UpdateContainerInstancesState(&ecs.UpdateContainerInstancesStateInput{
		Cluster:            aws.String(clusterName),
		ContainerInstances: aws.StringSlice([]string{instanceARN}),
		Status:             aws.String(ecs.ContainerInstanceStatusDraining),
	})
	if err != nil {
		return fmt.Errorf("failed to drain %s: %s", ec2InstanceID, err)
	}

	scheduler := time.NewTicker(10 * time.Second)
	timeout := time.NewTimer(5 * time.Minute)
	defer func() {
		scheduler.Stop()
		timeout.Stop()
	}()
	for {
		select {
		case <-scheduler.C:
			isDrained, err := isECSInstanceDrained(service, instanceARN, clusterName)
			if isDrained {
				return nil
			}
			if err != nil {
				fmt.Fprintf(out, "Warning: failed to get ECS instance state for %s: %s\n", ec2InstanceID, err)
			}
			fmt.Fprintf(out, "Waiting for %s to be drained\n", ec2InstanceID)
		case <-timeout.C:
			return fmt.Errorf("Timeout reached when trying to drain %s in %s ECS cluster", ec2InstanceID, clusterName)
		}
	}
}

func isECSInstanceDrained(service *ecs.ECS, instanceARN string, clusterName string) (bool, error) {
	instances, err := service.DescribeContainerInstances(&ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(clusterName),
		ContainerInstances: aws.StringSlice([]string{instanceARN}),
	})
	if err != nil {
		return false, err
	}

	instance := instances.ContainerInstances[0]

	return *instance.Status == ecs.ContainerInstanceStatusDraining && *instance.RunningTasksCount == 0 && *instance.PendingTasksCount == 0, nil
}

/*func rebalanceECSCluster(service *ecs.ECS, clusterName string) error {
	serviceARNs := []*string{}
	_, err := service.ListServicesPages(
		&ecs.ListServicesInput{Cluster: aws.String(clusterName)},
		func(page *ecs.ListServicesOutput, lastPage bool) bool {
			serviceARNs = append(serviceARNs, page.ServiceArns...)
			return true
		}
	),
	if err != nil {
		return fmt.Errorf("failed to get ECS services for %s: %s", clusterName, err)
	}

	if len(serviceARNs) == 0 {
		return nil
	}

	for i := 0; i < len(serviceARNs); i+= 100 {
		services, err := service.DescribeServices(&ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterName),
			Services: serviceARNs[i:min(i+100, len(serviceARNs))],
		})

		if err != nil {
			return fmt.Errorf("failed to get ECS services for %s: %s", clusterName, err)
		}

		taskList, err := service.ListTasksPages(
			&ecs.ListTasksInput
		)

	}

}*/

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
