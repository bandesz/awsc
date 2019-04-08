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

type MigrateService struct {
	asService  *autoscaling.AutoScaling
	ecsService *ecs.ECS
	out        io.Writer
}

func NewMigrateService(
	config *aws.Config,
	out io.Writer,
) *MigrateService {
	sess := session.Must(session.NewSession(config))
	return &MigrateService{
		asService:  autoscaling.New(sess),
		ecsService: ecs.New(sess),
		out:        out,
	}
}

// MigrateInstances replaces all the instances in an auto scaling group one-by-one
func (ms *MigrateService) MigrateInstances(asgName string, ecsClusterName string, minHealthyPercent int) error {
	ecsClusterInstances := map[string]string{}
	var err error
	if ecsClusterName != "" {
		ecsClusterInstances, err = ms.getECSClusterInstances(ecsClusterName)
		if err != nil {
			return fmt.Errorf("failed to get ECS container instances for %s: %s", ecsClusterName, err)
		}
	}

	oldInstances, err := ms.getAutoScalingGroupInstances(asgName)
	if err != nil {
		return err
	}
	instanceCount := len(oldInstances)

	if instanceCount == 0 {
		fmt.Fprintln(ms.out, "There are no instances in the auto scaling group.")
		return nil
	}

	maxInFlight := (100 - minHealthyPercent) * instanceCount / 100

	if maxInFlight == 0 {
		return fmt.Errorf("it is not possible to keep the minimum %d%% of instances healthy for %d instances, please lower the min-healthy-percent parameter", minHealthyPercent, instanceCount)
	}

	oldInstanceIDs := make(map[string]bool, instanceCount)
	for _, instance := range oldInstances {
		oldInstanceIDs[*instance.InstanceId] = true
	}

	inFlight := make(chan struct{}, maxInFlight)
	for i := 0; i < maxInFlight; i++ {
		inFlight <- struct{}{}
	}

	instancesToProcess := make(chan string, instanceCount)
	drained := make(chan string, maxInFlight)
	errors := make(chan error, maxInFlight)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	newInstances := make(map[string]bool, instanceCount)
	deletedInstanceCount := 0

	lastProgressTime := time.Now()

	fmt.Fprintf(ms.out, "Migrating %d instances, max in flight: %d\n", instanceCount, maxInFlight)

	for {
		select {
		case <-inFlight:
			if len(oldInstances) == 0 {
				continue
			}
			instancesToProcess <- *oldInstances[0].InstanceId
			oldInstances = oldInstances[1:]
		case instanceID := <-instancesToProcess:
			lastProgressTime = time.Now()
			if ecsInstance, isMember := ecsClusterInstances[instanceID]; isMember {
				go func() {
					err := ms.drainECSInstance(ecsClusterName, instanceID, ecsInstance)
					if err != nil {
						errors <- fmt.Errorf("failed to drain instance %s: %s", instanceID, err)
						time.AfterFunc(10*time.Second, func() {
							instancesToProcess <- instanceID
						})
						return
					}
					drained <- instanceID
				}()
			} else {
				drained <- instanceID
			}
		case instanceID := <-drained:
			fmt.Fprintf(ms.out, "Terminating %s\n", instanceID)
			_, err := ms.asService.TerminateInstanceInAutoScalingGroup(
				&autoscaling.TerminateInstanceInAutoScalingGroupInput{
					InstanceId:                     aws.String(instanceID),
					ShouldDecrementDesiredCapacity: aws.Bool(false),
				},
			)
			if err != nil {
				errors <- fmt.Errorf("failed to terminate instance %s: %s", instanceID, err.Error())
				time.AfterFunc(10*time.Second, func() {
					drained <- instanceID
				})
			}
		case <-ticker.C:
			instances, err := ms.getAutoScalingGroupInstances(asgName)
			if err != nil {
				errors <- fmt.Errorf("failed to get instances for %s", asgName)
				continue
			}

			healthyInstanceCount := 0
			oldInstanceCount := 0
			for _, instance := range instances {
				_, isOld := oldInstanceIDs[*instance.InstanceId]
				if isOld {
					oldInstanceCount++
				}

				instanceReady, err := ms.isInstanceReady(instance, ecsClusterName)
				if err != nil {
					errors <- fmt.Errorf("failed to check instance readiness for %s in %s", *instance.InstanceId, ecsClusterName)
					continue
				}
				if instanceReady {
					healthyInstanceCount++
					if !isOld {
						if _, registered := newInstances[*instance.InstanceId]; !registered {
							fmt.Fprintf(ms.out, "New instance is in service: %s\n", *instance.InstanceId)
							newInstances[*instance.InstanceId] = true
						}
					}
				}
			}

			addInFlight := min(
				instanceCount-oldInstanceCount-deletedInstanceCount,
				maxInFlight-(len(instances)-healthyInstanceCount),
			)
			for i := 0; i < addInFlight; i++ {
				inFlight <- struct{}{}
				deletedInstanceCount++
			}

			if healthyInstanceCount == len(instances) && oldInstanceCount == 0 {
				fmt.Fprintln(ms.out, "Finished.")
				return nil
			}

			if time.Now().After(lastProgressTime.Add(15 * time.Minute)) {
				return fmt.Errorf("timeout reached as no progress happened in 15 minutes")
			}
		case err := <-errors:
			fmt.Fprintf(ms.out, "Error: %s\n", err)
		}
	}
}

func (ms *MigrateService) getAutoScalingGroupInstances(asgName string) ([]*autoscaling.Instance, error) {
	output, err := ms.asService.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{asgName}),
	})
	if err != nil {
		return nil, err
	}
	if len(output.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("auto scaling group does not exist: %s", asgName)
	}
	return output.AutoScalingGroups[0].Instances, nil
}

func (ms *MigrateService) getECSClusterInstances(clusterName string) (map[string]string, error) {
	instanceARNs := []*string{}
	err := ms.ecsService.ListContainerInstancesPages(
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
		instances, err := ms.ecsService.DescribeContainerInstances(&ecs.DescribeContainerInstancesInput{
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

func (ms *MigrateService) drainECSInstance(clusterName string, ec2InstanceID string, instanceARN string) error {
	fmt.Fprintf(ms.out, "Draining %s in ECS cluster %s\n", ec2InstanceID, clusterName)
	_, err := ms.ecsService.UpdateContainerInstancesState(&ecs.UpdateContainerInstancesStateInput{
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
			isDrained, err := ms.isECSInstanceDrained(clusterName, instanceARN)
			if isDrained {
				return nil
			}
			if err != nil {
				fmt.Fprintf(ms.out, "Warning: failed to get ECS instance state for %s: %s\n", ec2InstanceID, err)
			}
		case <-timeout.C:
			return fmt.Errorf("Timeout reached when trying to drain %s in %s ECS cluster", ec2InstanceID, clusterName)
		}
	}
}

func (ms *MigrateService) isECSInstanceDrained(clusterName string, instanceARN string) (bool, error) {
	instances, err := ms.ecsService.DescribeContainerInstances(&ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(clusterName),
		ContainerInstances: aws.StringSlice([]string{instanceARN}),
	})
	if err != nil {
		return false, err
	}

	instance := instances.ContainerInstances[0]

	return *instance.Status == ecs.ContainerInstanceStatusDraining && *instance.RunningTasksCount == 0 && *instance.PendingTasksCount == 0, nil
}

func (ms *MigrateService) isInstanceReady(instance *autoscaling.Instance, clusterName string) (bool, error) {
	if isHealthyInstance(instance) {
		if clusterName == "" {
			return true, nil
		}

		return ms.isRegisteredECSHost(*instance.InstanceId, clusterName)
	}
	return false, nil
}

func isHealthyInstance(instance *autoscaling.Instance) bool {
	return *instance.LifecycleState == autoscaling.LifecycleStateInService &&
		*instance.HealthStatus == "Healthy"
}

func (ms *MigrateService) isRegisteredECSHost(instanceARN string, clusterName string) (bool, error) {
	ecsClusterInstances, err := ms.getECSClusterInstances(clusterName)
	if err != nil {
		return false, err
	}

	_, registered := ecsClusterInstances[instanceARN]
	return registered, nil
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
