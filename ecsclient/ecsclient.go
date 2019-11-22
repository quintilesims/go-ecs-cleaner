package ecsclient

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type Flags struct {
	Apply    bool
	Cutoff   int
	Debug    bool
	Parallel int
	Quiet    bool
	Verbose  bool
}

type ECSClient struct {
	Flags *Flags
	Svc   ECSSvc
}

func NewECSClient() *ECSClient {
	return &ECSClient{}
}

func (e *ECSClient) CleanupTaskDefinitions() error {
	clusterARNs, err := e.CollectClusters()
	if err != nil {
		return err
	}

	serviceARNsByClusterARN, err := e.CollectServices(clusterARNs)
	if err != nil {
		return err
	}

	ecsServices, err := e.DescribeServices(serviceARNsByClusterARN)
	if err != nil {
		return err
	}

	allTaskDefinitionARNs, err := e.CollectTaskDefinitions(ecsServices)
	if err != nil {
		return err
	}

	filteredTaskDefinitionARNs, err := e.FilterTaskDefinitions(allTaskDefinitionARNs, ecsServices)
	if err != nil {
		return err
	}

	if len(filteredTaskDefinitionARNs) > 0 {
		if e.Flags.Apply {
			if !e.Flags.Quiet {
				fmt.Printf("`--apply` flag present, deregistering %d task definitions...\n", len(filteredTaskDefinitionARNs))
			}

			if err = e.DeregisterTaskDefinitions(filteredTaskDefinitionARNs); err != nil {
				return err
			}

		} else {
			if !e.Flags.Quiet {
				fmt.Println("This is a dry run.")
				fmt.Println("Use the `--apply` flag to deregister these task definitions.")
			}
		}
	}

	if !e.Flags.Quiet {
		fmt.Println("Process finished.")
	}

	return nil
}

func (e *ECSClient) CollectClusters() ([]string, error) {
	if !e.Flags.Quiet {
		fmt.Println("Collecting clusters...")
	}

	var clusterARNs []string
	var nextToken *string
	var needToResetPrinter bool

	runPaginatedLoop := func() {
		var listedARNs []string
		var err error

		listedARNs, nextToken, err = e.listClusters(nextToken)
		if err != nil {
			if needToResetPrinter {
				fmt.Println()
				needToResetPrinter = false
			}

			fmt.Println("Error listing clusters:", err)
		}

		for _, arn := range listedARNs {
			clusterARNs = append(clusterARNs, arn)
		}

		if !e.Flags.Quiet {
			fmt.Printf("\r(found %d)", len(clusterARNs))
			needToResetPrinter = true
		}
	}

	runPaginatedLoop()
	for nextToken != nil {
		runPaginatedLoop()
	}

	if needToResetPrinter {
		fmt.Println()
	}

	return clusterARNs, nil
}

func (e *ECSClient) listClusters(nextToken *string) ([]string, *string, error) {
	listClustersInput := &ecs.ListClustersInput{
		NextToken: nextToken,
	}

	listClustersOutput, err := e.Svc.ListClusters(listClustersInput)
	if err != nil {
		return []string{}, nil, err
	}

	var clusterARNs []string
	for _, clusterARN := range listClustersOutput.ClusterArns {
		clusterARNs = append(clusterARNs, *clusterARN)
	}

	nextToken = listClustersOutput.NextToken
	return clusterARNs, nextToken, nil
}

func (e *ECSClient) CollectServices(clusterARNs []string) (map[string][]string, error) {
	if !e.Flags.Quiet {
		fmt.Println("Collecting services...")
	}

	var serviceARNsByClusterARN map[string][]string
	var numServices int
	var nextToken *string
	var needToResetPrinter bool

	runPaginatedLoop := func(clusterARN string) {
		var listedServiceARNs []string
		var err error

		listedServiceARNs, nextToken, err = e.listServices(clusterARN, nextToken)
		if err != nil {
			if needToResetPrinter {
				fmt.Println()
				needToResetPrinter = false
			}

			fmt.Println("Error listing services:", err)
		}

		for _, serviceARN := range listedServiceARNs {
			serviceARNsByClusterARN[clusterARN] = append(serviceARNsByClusterARN[clusterARN], serviceARN)
			numServices++
		}

		if !e.Flags.Quiet {
			fmt.Printf("\r(found %d)", numServices)
			needToResetPrinter = true
		}
	}

	for _, clusterARN := range clusterARNs {
		runPaginatedLoop(clusterARN)
		for nextToken != nil {
			runPaginatedLoop(clusterARN)
		}
	}

	if needToResetPrinter {
		fmt.Println()
	}

	return serviceARNsByClusterARN, nil
}

func (e *ECSClient) listServices(clusterArn string, nextToken *string) ([]string, *string, error) {
	listServicesInput := &ecs.ListServicesInput{
		Cluster:   aws.String(clusterArn),
		NextToken: nextToken,
	}

	listServicesOutput, err := e.Svc.ListServices(listServicesInput)
	if err != nil {
		return []string{}, nil, err
	}

	var serviceArns []string
	for _, arn := range listServicesOutput.ServiceArns {
		serviceArns = append(serviceArns, *arn)
	}

	nextToken = listServicesOutput.NextToken
	return serviceArns, nextToken, nil
}

func (e *ECSClient) CollectTaskDefinitions([]ecs.Service) ([]string, error) {
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
}

func (e *ECSClient) ConfigureSession() error {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return err
	}

	e.Svc = ecs.New(sess)
	return nil
}

func (e *ECSClient) DeregisterTaskDefinitions([]string) error {
	if len(taskDefinitionArns) < parallel {
		parallel = len(taskDefinitionArns)
	}

	jobsChan := make(chan Job, parallel) // closed by dispatcher
	resultsChan := make(chan Result, parallel)
	quitChan := make(chan bool, parallel)

	defer close(resultsChan)
	defer close(quitChan)

	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go worker(svc, &wg, jobsChan, resultsChan, quitChan)
	}

	wg.Add(1)
	go dispatcher(&wg, taskDefinitionArns, parallel, jobsChan, resultsChan, quitChan, verbose, debug)

	wg.Wait()
}

func (e *ECSClient) DescribeServices(map[string][]string) ([]ecs.Service, error) {
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
}

func (e *ECSClient) FilterTaskDefinitions(allTaskDefinitionARNs []string, ecsServices []ecs.Service) ([]string, error) {
	// filter out in-use/active task defs

	var inUseTaskDefinitionArns []string

	for _, service := range ecsServices {
		inUseTaskDefinitionArns = append(inUseTaskDefinitionArns, *service.TaskDefinition)
	}

	if !e.Flags.Quiet {
		fmt.Printf("Filtering out %d in-use task definitions\n", len(inUseTaskDefinitionArns))
	}

	if e.Flags.Verbose {
		for _, arn := range inUseTaskDefinitionArns {
			fmt.Println(arn)
		}
	}

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

			if flags["verbose"].(bool) {
				fmt.Printf("\r(found %d)", len(familyTaskDefinitionArns))
				needToResetPrinter = true
			}
		}

		if flags["verbose"].(bool) {
			if needToResetPrinter {
				fmt.Println()
				needToResetPrinter = false
			} else {
				fmt.Printf("(found %d)\n", len(familyTaskDefinitionArns))
			}
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
	if flags["verbose"].(bool) {
		for _, arn := range mostRecentActiveTaskDefinitionArns {
			fmt.Println(arn)
		}
	}

	allTaskDefinitionArns = removeAFromB(mostRecentActiveTaskDefinitionArns, allTaskDefinitionArns)
}
