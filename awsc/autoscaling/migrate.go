package autoscaling

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

// MigrateInstances replaces all the instances in an auto scaling group one-by-one
func MigrateInstances(config *aws.Config, out io.Writer, name string) error {
	sess := session.Must(session.NewSession(config))
	service := autoscaling.New(sess)

	group, err := getAutoScalingGroup(service, name)
	if err != nil {
		return err
	}

	oldInstances := group.Instances
	instanceCount := len(oldInstances)
	fmt.Fprintf(out, "Instance count: %d\n", instanceCount)

	for _, oldInstance := range oldInstances {
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
			time.Sleep(20 * time.Second)

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
