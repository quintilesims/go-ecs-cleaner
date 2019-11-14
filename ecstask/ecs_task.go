package ecstask

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/spf13/cobra"
)

// Run is the entrypoint used by the CLI for this set of work.
func Run(cmd *cobra.Command, args []string) {
	fmt.Println("running ecs-task")

	// configure AWS connection

	fmt.Println("configuring session...")

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	})
	if err != nil {
		fmt.Println("Error creating AWS session ", err)
		return
	}

	svc := ecs.New(sess)

	// list all clusters

	fmt.Println("collecting clusters...")

	listClustersInput := &ecs.ListClustersInput{}

	ecsClusters, err := svc.ListClusters(listClustersInput)
	if err != nil {
		fmt.Println("Error listing clusters ", err)
		return
	}

	var clusterArns []string

	for _, ecsClusterArn := range ecsClusters.ClusterArns {
		clusterArns = append(clusterArns, *ecsClusterArn)
	}

	// describe all services
	// - list services per cluster
	// - describe each service

	fmt.Println("collecting services...")

	var serviceArns []string

	for _, clusterArn := range clusterArns {
		listServices := func(clusterArn string, serviceArns []string, nextToken *string) ([]string, *string) {
			listServicesInput := &ecs.ListServicesInput{
				Cluster: aws.String(clusterArn),
			}

			listServicesOutput, err := svc.ListServices(listServicesInput)
			if err != nil {
				fmt.Println("Error listing services, ", err)
				return serviceArns, nil
			}

			for _, arn := range listServicesOutput.ServiceArns {
				serviceArns = append(serviceArns, *arn)
			}

			nextToken = listServicesOutput.NextToken
			return serviceArns, nextToken
		}

		var nextToken *string

		serviceArns, nextToken = listServices(clusterArn, serviceArns, nextToken)

		for nextToken != nil {
			serviceArns, nextToken = listServices(clusterArn, serviceArns, nextToken)
		}
	}

	fmt.Println("services found:", serviceArns)

	// describeServicesInput := &ecs.DescribeServicesInput{}

	// ecsServices, err := svc.DescribeServices(describeServicesInput)
	// if err != nil {
	// 	fmt.Println("Error describing services, ", err)
	// 	return
	// }

	// gather task definitions

	// filter out in-use/active task defs

	// filter out n most-recent per family

	// what's left will be removed

	// dry-run vs apply

	// if apply, deregister task defs
}
