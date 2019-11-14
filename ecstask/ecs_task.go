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

	dryRun := true

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

	clusterServiceArns := make(map[string][]string)

	for _, ecsClusterArn := range ecsClusters.ClusterArns {
		clusterServiceArns[*ecsClusterArn] = []string{}
	}

	// list services per cluster

	fmt.Println("collecting services...")

	for clusterArn := range clusterServiceArns {

		var nextToken *string

		serviceArns, nextToken := listServices(svc, clusterArn, nextToken)
		for _, arn := range serviceArns {
			clusterServiceArns[clusterArn] = append(clusterServiceArns[clusterArn], arn)
		}

		for nextToken != nil {
			serviceArns, nextToken = listServices(svc, clusterArn, nextToken)

			for _, arn := range serviceArns {
				clusterServiceArns[clusterArn] = append(clusterServiceArns[clusterArn], arn)
			}
		}
	}

	// describe each service

	fmt.Println("describing services...")

	clusterServices := make(map[string][]ecs.Service)
	limit := 10

	for clusterArn, serviceArns := range clusterServiceArns {
		for len(serviceArns) > 0 {
			length := len(serviceArns)
			var (
				serviceArnsChunk []string
				iStart           int
				iEnd             int
			)

			if length >= limit {
				iStart = length - limit
				iEnd = length
			} else {
				iStart = 0
				iEnd = length
			}

			serviceArnsChunk = serviceArns[iStart:iEnd]
			serviceArns = serviceArns[0:iStart]

			services := describeServices(svc, clusterArn, serviceArnsChunk)

			var serviceCollection []ecs.Service

			for _, service := range services {
				serviceCollection = append(serviceCollection, service)
			}

			clusterServices[clusterArn] = serviceCollection
		}
	}

	// gather task definitions

	// filter out in-use/active task defs

	// filter out n most-recent per family

	// what's left will be removed

	// dry-run vs apply

	// if apply, deregister task defs

	if dryRun == true {
		fmt.Println("dry-run")
	} else {
		fmt.Println("not a dry-run!")
	}
}

func listServices(svc *ecs.ECS, clusterArn string, nextToken *string) ([]string, *string) {
	listServicesInput := &ecs.ListServicesInput{
		Cluster: aws.String(clusterArn),
	}

	listServicesOutput, err := svc.ListServices(listServicesInput)
	if err != nil {
		fmt.Println("Error listing services, ", err)
		return []string{}, nil
	}

	var serviceArns []string
	for _, arn := range listServicesOutput.ServiceArns {
		serviceArns = append(serviceArns, *arn)
	}

	nextToken = listServicesOutput.NextToken
	return serviceArns, nextToken
}

func describeServices(svc *ecs.ECS, clusterArn string, serviceArns []string) []ecs.Service {
	var inputServices []*string

	for _, serviceArn := range serviceArns {
		inputServices = append(inputServices, aws.String(serviceArn))
	}

	describeServicesInput := &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterArn),
		Services: inputServices,
	}

	ecsServices, err := svc.DescribeServices(describeServicesInput)
	if err != nil {
		fmt.Println("Error describing services, ", err)
		return []ecs.Service{}
	}

	var services []ecs.Service

	for _, ecsService := range ecsServices.Services {
		services = append(services, *ecsService)
	}

	return services
}
