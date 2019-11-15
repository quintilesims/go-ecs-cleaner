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

	apply := false

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

	var nextToken *string
	var clusterArns []string

	clusters, nextToken := listClusters(svc, nextToken)
	for _, arn := range clusters {
		clusterArns = append(clusterArns, arn)
	}

	needToResetPrinter := false
	for nextToken != nil {
		clusters, nextToken = listClusters(svc, nextToken)

		for _, arn := range clusters {
			clusterArns = append(clusterArns, arn)
		}

		fmt.Printf("\r(found %d)", len(clusterArns))
		needToResetPrinter = true
	}

	if needToResetPrinter {
		fmt.Println()
		needToResetPrinter = false
	} else {
		fmt.Printf("(found %d)\n", len(clusterArns))
	}

	clusterServiceArns := make(map[string][]string)

	for _, clusterArn := range clusterArns {
		clusterServiceArns[clusterArn] = []string{}
	}

	// list services per cluster

	fmt.Println("collecting services...")

	var numServices int
	nextToken = nil
	for clusterArn := range clusterServiceArns {
		serviceArns, nextToken := listServices(svc, clusterArn, nextToken)
		for _, arn := range serviceArns {
			clusterServiceArns[clusterArn] = append(clusterServiceArns[clusterArn], arn)
			numServices++
		}

		for nextToken != nil {
			serviceArns, nextToken = listServices(svc, clusterArn, nextToken)

			for _, arn := range serviceArns {
				clusterServiceArns[clusterArn] = append(clusterServiceArns[clusterArn], arn)
				numServices++
			}

			fmt.Printf("\r(found %d)", numServices)
			needToResetPrinter = true
		}
	}

	if needToResetPrinter {
		fmt.Println()
		needToResetPrinter = false
	} else {
		fmt.Printf("(found %d)\n", numServices)
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

	fmt.Println("collecting task definitions...")

	nextToken = nil
	var allTaskDefinitionArns []string

	taskDefinitionArns, nextToken := listTaskDefinitions(svc, nextToken)
	for _, arn := range taskDefinitionArns {
		allTaskDefinitionArns = append(allTaskDefinitionArns, arn)
	}

	needToResetPrinter = false
	for nextToken != nil {
		taskDefinitionArns, nextToken = listTaskDefinitions(svc, nextToken)

		for _, arn := range taskDefinitionArns {
			allTaskDefinitionArns = append(allTaskDefinitionArns, arn)
		}

		fmt.Printf("\r(found %d)", len(allTaskDefinitionArns))
		needToResetPrinter = true
	}

	if needToResetPrinter {
		fmt.Println()
		needToResetPrinter = false
	} else {
		fmt.Printf("(found %d)\n", len(allTaskDefinitionArns))
	}

	// filter out in-use/active task defs

	var inUseTaskDefinitionArns []string

	for _, services := range clusterServices {
		for _, service := range services {
			inUseTaskDefinitionArns = append(inUseTaskDefinitionArns, *service.TaskDefinition)
		}
	}

	fmt.Printf("filtering out %d in-use task definitions\n", len(inUseTaskDefinitionArns))

	allTaskDefinitionArns = difference(inUseTaskDefinitionArns, allTaskDefinitionArns)

	fmt.Printf("%d task definitions remain\n", len(allTaskDefinitionArns))

	// filter out n most-recent per family

	// what's left will be removed

	// dry-run vs apply

	// if apply, deregister task defs

	if apply == true {
		fmt.Println("not a dry-run!")
	} else {
		fmt.Println("dry-run, no action taken")
	}
}

func listClusters(svc *ecs.ECS, nextToken *string) ([]string, *string) {
	listClustersInput := &ecs.ListClustersInput{
		NextToken: nextToken,
	}

	listClustersOutput, err := svc.ListClusters(listClustersInput)
	if err != nil {
		fmt.Println("Error listing clusters ", err)
		return []string{}, nil
	}

	var clusterArns []string
	for _, arn := range listClustersOutput.ClusterArns {
		clusterArns = append(clusterArns, *arn)
	}

	nextToken = listClustersOutput.NextToken
	return clusterArns, nextToken
}

func listServices(svc *ecs.ECS, clusterArn string, nextToken *string) ([]string, *string) {
	listServicesInput := &ecs.ListServicesInput{
		Cluster:   aws.String(clusterArn),
		NextToken: nextToken,
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

func listTaskDefinitions(svc *ecs.ECS, nextToken *string) ([]string, *string) {
	listTaskDefinitionsInput := &ecs.ListTaskDefinitionsInput{
		NextToken: nextToken,
	}

	listTaskDefinitionsOutput, err := svc.ListTaskDefinitions(listTaskDefinitionsInput)
	if err != nil {
		fmt.Println("Error listing task definitions,", err)
		return []string{}, nil
	}

	var taskDefinitionArns []string
	for _, arn := range listTaskDefinitionsOutput.TaskDefinitionArns {
		taskDefinitionArns = append(taskDefinitionArns, *arn)
	}

	nextToken = listTaskDefinitionsOutput.NextToken
	return taskDefinitionArns, nextToken
}

func difference(a, b []string) []string {
	var diff []string
	m := make(map[string]int)

	for _, item := range a {
		m[item]++
	}

	for _, item := range b {
		m[item]++
	}

	for k, v := range m {
		if v == 1 {
			diff = append(diff, k)
		}
	}

	return diff
}
