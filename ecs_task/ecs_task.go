package ecs_task

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/spf13/cobra"
)

func Run(cmd *cobra.Command, args []string) {
	fmt.Println("running ecs-task")

	// configure AWS connection

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	})
	if err != nil {
		fmt.Println("Error creating AWS session ", err)
		return
	}

	svc := ecs.New(sess)

	// list all clusters

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

	fmt.Println(clusterArns)

	// describe all services

	// gather task definitions

	// filter out in-use/active task defs

	// filter out n most-recent per family

	// what's left will be removed

	// dry-run vs apply

	// if apply, deregister task defs
}
