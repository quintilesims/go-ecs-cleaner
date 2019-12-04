package ecsclient

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang-collections/collections/stack"
	"github.com/jpillora/backoff"
)

// Flags hold user-defined operational parameters for the ECSClient.
// They are specified at the command line when `go-ecs-client ecs-task` is run.
type Flags struct {
	Apply   bool
	Cutoff  int
	Debug   bool
	Quiet   bool
	Verbose bool
}

// ECSClient is the object through which the `ecs-task` command interacts with AWS.
type ECSClient struct {
	Flags Flags
	Svc   ECSSvc
}

// NewECSClient creates an ECSClient and returns a pointer to it.
func NewECSClient() *ECSClient {
	return &ECSClient{}
}

// CleanupTaskDefinitions defines the overarching logic workflow for cleaning up task definitions.
func (e *ECSClient) CleanupTaskDefinitions() error {
	allTaskDefinitionARNs, err := e.CollectTaskDefinitions()
	if err != nil {
		return err
	}

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

// CollectClusters gathers the ARNs of all the clusters for the configured account and region.
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

// CollectServices gathers the ARNs of all the services associated with the clusters
// that are passed in for the configured account and region.
func (e *ECSClient) CollectServices(clusterARNs []string) (map[string][]string, error) {
	if !e.Flags.Quiet {
		fmt.Println("Collecting services...")
	}

	serviceARNsByClusterARN := make(map[string][]string)
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

// CollectTaskDefinitions gathers the ARNs of all the task definitions for the configured
// account and region.
func (e *ECSClient) CollectTaskDefinitions() ([]string, error) {
	if !e.Flags.Quiet {
		fmt.Println("Collecting task definitions...")
	}

	var taskDefinitionARNs []string
	var nextToken *string
	var needToResetPrinter bool

	runPaginatedLoop := func() {
		var listedTaskDefinitionARNs []string
		var err error

		listedTaskDefinitionARNs, nextToken, err = e.listTaskDefinitions("", "", nextToken)
		if err != nil {
			if needToResetPrinter {
				fmt.Println()
				needToResetPrinter = false
			}

			fmt.Println("Error listing task definitions:", err)
		}

		for _, taskDefinitionARN := range listedTaskDefinitionARNs {
			taskDefinitionARNs = append(taskDefinitionARNs, taskDefinitionARN)
		}

		if !e.Flags.Quiet {
			fmt.Printf("\r(found %d)", len(taskDefinitionARNs))
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

	return taskDefinitionARNs, nil
}

// ConfigureSession configures and instantiates an `ecs.ECS` object into the ECSClient's
// `Svc` field. This `ecs.ECS` object satisfies the `ECSSvc` interface defined in this package.
func (e *ECSClient) ConfigureSession() error {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return err
	}

	e.Svc = ecs.New(sess)
	return nil
}

// DeregisterTaskDefinitions creates the dispatchDeregistrationJobs and doDeregistrationJobs goroutines that send requests
// to the AWS API to deregister given task definition ARNs.
func (e *ECSClient) DeregisterTaskDefinitions(taskDefinitionARNs []string) error {
	arns := stack.New()
	for _, taskDefinitionARN := range taskDefinitionARNs {
		arns.Push(taskDefinitionARN)
	}

	var failedDeregistrations []FailedDeregistration
	var numCompletedDeregistrations int
	numTasksToDeregister := len(taskDefinitionARNs)
	var needToResetPrinter bool

	b := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    2 * time.Minute,
		Jitter: true,
	}

	for numCompletedDeregistrations < numTasksToDeregister {
		arn := arns.Pop().(string)
		input := &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(arn),
		}

		if _, err := e.Svc.DeregisterTaskDefinition(input); err != nil {
			switch {

			case e.isThrottlingError(err):
				t := b.Duration()

				if e.Flags.Verbose {
					if needToResetPrinter {
						fmt.Println()
						needToResetPrinter = false
					}

					fmt.Printf("Backoff triggered for %s\n", arn)

					if e.Flags.Debug {
						fmt.Printf("Triggering error: %v\n", err)
					}

					fmt.Printf("Waiting for %v\n", t)
				}

				time.Sleep(t)
				arns.Push(arn)

			case e.isExpiredTokenError(err):
				if e.Flags.Verbose {
					if needToResetPrinter {
						fmt.Println()
						needToResetPrinter = false
					}

					fmt.Println("Token expired, creating new session.")
				}

				e.ConfigureSession()
				arns.Push(arn)

			case e.isStopworthyError(err):
				if needToResetPrinter {
					fmt.Println()
					needToResetPrinter = false
				}

				fmt.Println("Encountered stopworthy error, halting process.")
				return err

			default:
				failedDeregistration := FailedDeregistration{Arn: arn, Err: err}
				failedDeregistrations = append(failedDeregistrations, failedDeregistration)
				numTasksToDeregister--
			}

		} else {
			b.Reset()
			numCompletedDeregistrations++
		}

		fmt.Printf("\r%d deregistered task definitions, %d errored", numCompletedDeregistrations, len(failedDeregistrations))
		needToResetPrinter = true
	}

	if needToResetPrinter {
		fmt.Println()
		needToResetPrinter = false
	}

	if e.Flags.Verbose && len(failedDeregistrations) > 0 {
		fmt.Println("Errored task definition deregistrations:")
		for _, result := range failedDeregistrations {
			if e.Flags.Debug {
				fmt.Printf("%s (%v)\n", result.Arn, result.Err)
			} else {
				fmt.Printf("%s\n", result.Arn)
			}
		}
	}

	return nil
}

// DescribeServices compiles a list of `ecs.Service` objects given a map of cluster ARNs to
// lists of service ARNs associated with each cluster. Most importantly, these `ecs.Service`
// objects contain the ARNs of the task definitions currently in use by the services.
func (e *ECSClient) DescribeServices(serviceARNsByClusterARN map[string][]string) ([]ecs.Service, error) {
	if !e.Flags.Quiet {
		fmt.Println("Describing services...")
	}

	var ecsServices []ecs.Service

	limit := 10
	for clusterARN, serviceARNs := range serviceARNsByClusterARN {
		for len(serviceARNs) > 0 {
			length := len(serviceARNs)
			var (
				serviceARNsChunk []string
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

			serviceARNsChunk = serviceARNs[iStart:iEnd]
			serviceARNs = serviceARNs[0:iStart]

			describedServices, err := e.describeServices(clusterARN, serviceARNsChunk)
			if err != nil {
				fmt.Println("Error describing services:", err)
			}

			for _, describedService := range describedServices {
				ecsServices = append(ecsServices, describedService)
			}
		}
	}

	return ecsServices, nil
}

// FilterTaskDefinitions takes a master list of task definition ARNs and returns a version of
// that list from which has been removed:
//   - All task definitions curently in use by a service.
//   - All task definitions which are among the `n`-most-recently-used task definitions for each
//     family. `n` is configured via the `--cutoff` flag.
func (e *ECSClient) FilterTaskDefinitions(allTaskDefinitionARNs []string, ecsServices []ecs.Service) ([]string, error) {
	// filter out in-use/active task defs

	var inUseTaskDefinitionARNs []string

	for _, service := range ecsServices {
		inUseTaskDefinitionARNs = append(inUseTaskDefinitionARNs, *service.TaskDefinition)
	}

	if !e.Flags.Quiet {
		fmt.Printf("Filtering out %d in-use task definitions\n", len(inUseTaskDefinitionARNs))
	}

	if e.Flags.Verbose {
		for _, arn := range inUseTaskDefinitionARNs {
			fmt.Println(arn)
		}
	}

	allTaskDefinitionARNs = removeAFromB(inUseTaskDefinitionARNs, allTaskDefinitionARNs)

	if !e.Flags.Quiet {
		fmt.Printf("%d task definitions remain\n", len(allTaskDefinitionARNs))
	}

	// filter out n most-recent per family

	var inUseTaskDefinitionFamilies []string

	for _, arn := range inUseTaskDefinitionARNs {
		r1 := regexp.MustCompile(`([A-Za-z0-9_-]+):([0-9]+)$`)
		r2 := regexp.MustCompile(`^([A-Za-z0-9_-]+):`)
		family := strings.TrimSuffix(r2.FindString(r1.FindString(arn)), ":")
		inUseTaskDefinitionFamilies = append(inUseTaskDefinitionFamilies, family)
	}

	if !e.Flags.Quiet {
		fmt.Println("Collecting active-family task definitions...")
	}

	var mostRecentFamilyTaskDefinitionARNs []string
	var allFamilyTaskDefinitionARNs []string

	var nextToken *string
	var needToResetPrinter bool

	runPaginatedLoop := func(taskDefinitionFamily string) {
		var listedTaskDefinitionARNs []string
		var err error

		listedTaskDefinitionARNs, nextToken, err = e.listTaskDefinitions(taskDefinitionFamily, "DESC", nextToken)
		if err != nil {
			fmt.Println("Error listing task definitions:", err)
		}

		for _, listedTaskDefinitionARN := range listedTaskDefinitionARNs {
			allFamilyTaskDefinitionARNs = append(allFamilyTaskDefinitionARNs, listedTaskDefinitionARN)
		}

		if e.Flags.Verbose {
			fmt.Printf("\r(found %d)", len(allFamilyTaskDefinitionARNs))
			needToResetPrinter = true
		}

	}

	for _, taskDefinitionFamily := range inUseTaskDefinitionFamilies {
		runPaginatedLoop(taskDefinitionFamily)
		for nextToken != nil {
			runPaginatedLoop(taskDefinitionFamily)
		}
	}

	if e.Flags.Verbose {
		if needToResetPrinter {
			fmt.Println()
			needToResetPrinter = false
		} else {
			fmt.Printf("(found %d)\n", len(allFamilyTaskDefinitionARNs))
		}
	}

	allFamilyTaskDefinitionARNs = removeAFromB(inUseTaskDefinitionARNs, allFamilyTaskDefinitionARNs)

	if len(allFamilyTaskDefinitionARNs) > e.Flags.Cutoff {
		allFamilyTaskDefinitionARNs = allFamilyTaskDefinitionARNs[0:e.Flags.Cutoff]
	}

	for _, taskDefinitionARN := range allFamilyTaskDefinitionARNs {
		mostRecentFamilyTaskDefinitionARNs = append(mostRecentFamilyTaskDefinitionARNs, taskDefinitionARN)
	}

	if !e.Flags.Quiet {
		fmt.Printf("filtering out %d recent task definitions across %d families\n", len(mostRecentFamilyTaskDefinitionARNs), len(inUseTaskDefinitionFamilies))
	}

	if e.Flags.Verbose {
		for _, arn := range mostRecentFamilyTaskDefinitionARNs {
			fmt.Println(arn)
		}
	}

	allTaskDefinitionARNs = removeAFromB(mostRecentFamilyTaskDefinitionARNs, allTaskDefinitionARNs)

	return allTaskDefinitionARNs, nil
}

// listClusters is a helper method that handles interaction with AWS objects.
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

// listServices is a helper method that handles interaction with AWS objects.
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

// listTaskDefinitions is a helper method that handles interaction with AWS objects.
func (e *ECSClient) listTaskDefinitions(familyPrefix, sort string, nextToken *string) ([]string, *string, error) {
	listTaskDefinitionsInput := &ecs.ListTaskDefinitionsInput{
		NextToken: nextToken,
	}

	if familyPrefix != "" {
		listTaskDefinitionsInput.SetFamilyPrefix(familyPrefix)
	}

	if sort != "" {
		listTaskDefinitionsInput.SetSort(sort)
	}

	listTaskDefinitionsOutput, err := e.Svc.ListTaskDefinitions(listTaskDefinitionsInput)
	if err != nil {
		return []string{}, nil, err
	}

	var taskDefinitionARNs []string
	for _, taskDefinitionARN := range listTaskDefinitionsOutput.TaskDefinitionArns {
		taskDefinitionARNs = append(taskDefinitionARNs, *taskDefinitionARN)
	}

	nextToken = listTaskDefinitionsOutput.NextToken
	return taskDefinitionARNs, nextToken, nil
}

// describeServices is a helper method that handles interaction with AWS objects.
func (e *ECSClient) describeServices(clusterARN string, serviceARNs []string) ([]ecs.Service, error) {
	var inputServices []*string

	for _, serviceARN := range serviceARNs {
		inputServices = append(inputServices, aws.String(serviceARN))
	}

	describeServicesInput := &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterARN),
		Services: inputServices,
	}

	ecsServices, err := e.Svc.DescribeServices(describeServicesInput)
	if err != nil {
		return []ecs.Service{}, err
	}

	var services []ecs.Service

	for _, ecsService := range ecsServices.Services {
		services = append(services, *ecsService)
	}

	return services, nil
}

// Checks whether a given error is the result of the ECS Service's session token
// having expired.
func (e *ECSClient) isExpiredTokenError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		code := awsErr.Code()

		if code == "ExpiredTokenException" {
			return true
		}
	}

	return false
}

// Checks whether a given error is something for which we would consider halting the
// entire process for.
func (e *ECSClient) isStopworthyError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		code := awsErr.Code()

		if code == "" {
			return true
		}
	}

	return false
}

// Checks whether a given error is something we would consider to be a throttling
// error.
func (e *ECSClient) isThrottlingError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		code := awsErr.Code()

		if code == "Throttling" || code == "ThrottlingException" {
			return true
		}

		message := strings.ToLower(awsErr.Message())
		if code == "ClientException" && strings.Contains(message, "too many concurrent attempts") {
			return true
		}
	}

	return false
}
