package ecstask

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/spf13/cobra"
)

// Run is the entrypoint used by the CLI for this set of work.
func Run(cmd *cobra.Command, args []string, flags map[string]interface{}) {
	fmt.Println("running ecs-task")

	// configure AWS connection

	fmt.Printf("configuring session for region %s...\n", flags["region"].(string))

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(flags["region"].(string)),
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

	taskDefinitionArns, nextToken := listTaskDefinitions(svc, "", "", nextToken)
	for _, arn := range taskDefinitionArns {
		allTaskDefinitionArns = append(allTaskDefinitionArns, arn)
	}

	needToResetPrinter = false
	for nextToken != nil {
		taskDefinitionArns, nextToken = listTaskDefinitions(svc, "", "", nextToken)

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

	allTaskDefinitionArns = removeAFromB(inUseTaskDefinitionArns, allTaskDefinitionArns)

	fmt.Printf("%d task definitions remain\n", len(allTaskDefinitionArns))

	// filter out n most-recent per family

	var inUseTaskDefinitionFamilies []string

	for _, arn := range inUseTaskDefinitionArns {
		r1 := regexp.MustCompile(`([A-Za-z0-9_-]+):([0-9]+)$`)
		r2 := regexp.MustCompile(`^([A-Za-z0-9_-]+):`)
		family := strings.TrimSuffix(r2.FindString(r1.FindString(arn)), ":")
		inUseTaskDefinitionFamilies = append(inUseTaskDefinitionFamilies, family)
	}

	fmt.Println("collecting active-family task definitions...")

	var mostRecentActiveTaskDefinitionArns []string

	nextToken = nil
	for _, family := range inUseTaskDefinitionFamilies {
		nextToken = nil
		needToResetPrinter = false
		var familyTaskDefinitionArns []string

		taskDefinitionArns, nextToken = listTaskDefinitions(svc, family, "DESC", nextToken)
		for _, arn := range taskDefinitionArns {
			familyTaskDefinitionArns = append(familyTaskDefinitionArns, arn)
		}

		for nextToken != nil {
			taskDefinitionArns, nextToken = listTaskDefinitions(svc, family, "DESC", nextToken)

			for _, arn := range taskDefinitionArns {
				familyTaskDefinitionArns = append(familyTaskDefinitionArns, arn)
			}

			fmt.Printf("\r(found %d)", len(familyTaskDefinitionArns))
			needToResetPrinter = true
		}

		if needToResetPrinter {
			fmt.Println()
			needToResetPrinter = false
		} else {
			fmt.Printf("(found %d)\n", len(familyTaskDefinitionArns))
		}

		familyTaskDefinitionArns = removeAFromB(inUseTaskDefinitionArns, familyTaskDefinitionArns)

		mostRecentCutoff := flags["cutoff"].(int)
		if len(familyTaskDefinitionArns) > mostRecentCutoff {
			familyTaskDefinitionArns = familyTaskDefinitionArns[0:mostRecentCutoff]
		}

		for _, arn := range familyTaskDefinitionArns {
			mostRecentActiveTaskDefinitionArns = append(mostRecentActiveTaskDefinitionArns, arn)
		}

	}

	fmt.Printf("filtering out %d recent task definitions across %d families\n", len(mostRecentActiveTaskDefinitionArns), len(inUseTaskDefinitionFamilies))

	allTaskDefinitionArns = removeAFromB(mostRecentActiveTaskDefinitionArns, allTaskDefinitionArns)

	fmt.Printf("%d task definitions ready to be deregistered\n", len(allTaskDefinitionArns))

	// what's left will be removed (unless dry-run)

	if flags["apply"].(bool) {
		fmt.Println("\n`--apply` flag present")
		fmt.Printf("deregistering %d task definitions...\n", len(allTaskDefinitionArns))

		deregisterTaskDefinitions(svc, allTaskDefinitionArns, flags["parallel"].(int))

		fmt.Println("finished")
	} else {
		fmt.Println("\nthis is a dry run")
		fmt.Println("use the `--apply` flag to deregister these task definitions")
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

func listTaskDefinitions(svc *ecs.ECS, familyPrefix, sort string, nextToken *string) ([]string, *string) {
	listTaskDefinitionsInput := &ecs.ListTaskDefinitionsInput{
		NextToken: nextToken,
	}

	if familyPrefix != "" {
		listTaskDefinitionsInput.SetFamilyPrefix(familyPrefix)
	}

	if sort != "" {
		listTaskDefinitionsInput.SetSort(sort)
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

func removeAFromB(a, b []string) []string {
	var diff []string
	m := make(map[string]int)

	for _, item := range b {
		m[item] = 1
	}

	for _, item := range a {
		if m[item] != 0 {
			m[item]++
		}
	}

	for k, v := range m {
		if v == 1 {
			diff = append(diff, k)
		}
	}

	return diff
}

func deregisterTaskDefinitions(svc *ecs.ECS, taskDefinitionArns []string, parallel int) {
	arnsChan := make(chan string, len(taskDefinitionArns))

	deregisterTaskDefinition := func(arn string) {
		_, err := svc.DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(arn),
		})
		if err != nil {
			fmt.Println("Error deregistering task definition:", err)
		}
	}

	worker := func(wg *sync.WaitGroup) {
		for arn := range arnsChan {
			deregisterTaskDefinition(arn)
		}

		wg.Done()
	}

	createWorkerPool := func(numWorkers int) {
		var wg sync.WaitGroup
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go worker(&wg)
		}

		wg.Wait()
	}

	allocate := func(arns []string) {
		for _, arn := range arns {
			arnsChan <- arn
		}

		close(arnsChan)
	}

	go allocate(taskDefinitionArns)
	createWorkerPool(parallel)
}
