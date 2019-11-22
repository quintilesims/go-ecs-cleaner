package ecstask

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang-collections/collections/stack"
	"github.com/jpillora/backoff"
	"github.com/spf13/cobra"
)

// Run is the entrypoint used by the CLI for this set of work.
func Run(cmd *cobra.Command, args []string, flags map[string]interface{}) {
	fmt.Println("running ecs-task")

	// list all clusters

	// list services per cluster

	// describe each service

	// gather task definitions

	fmt.Printf("%d task definitions to deregister\n", len(allTaskDefinitionArns))

	// what's left will be removed (unless dry-run)

}

func describeServices(svc EcsSvc, clusterArn string, serviceArns []string) []ecs.Service {
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

func listTaskDefinitions(svc EcsSvc, familyPrefix, sort string, nextToken *string) ([]string, *string) {
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

// Job carries information through a Job channel.
type Job struct {
	Arn string
}

// Result carries information through a Result channel.
type Result struct {
	Arn string
	Err error
}

func deregisterTaskDefinitions(svc EcsSvc, taskDefinitionArns []string, parallel int, verbose, debug bool) {
}

func worker(svc EcsSvc, wg *sync.WaitGroup, jobsChan <-chan Job, resultsChan chan<- Result, quitChan <-chan bool) {
	defer wg.Done()

	for job := range jobsChan {
		select {
		case <-quitChan:
			return
		default:
			input := &ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: aws.String(job.Arn),
			}

			_, err := svc.DeregisterTaskDefinition(input)

			result := Result{Arn: job.Arn, Err: err}
			resultsChan <- result
		}
	}
}

func dispatcher(wg *sync.WaitGroup, arns []string, parallel int, jobsChan chan Job, resultsChan chan Result, quitChan chan<- bool, verbose, debug bool) {
	defer wg.Done()
	defer close(jobsChan)

	jobs := stack.New()
	for _, arn := range arns {
		jobs.Push(Job{arn})
	}

	var failedJobs []Result
	var numCompletedJobs int
	numJobsToComplete := len(arns)

	preload := 1
	if parallel > 1 {
		preload = parallel - 1
	}

	for i := 0; i < preload; i++ {
		jobsChan <- jobs.Pop().(Job)
	}

	b := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    2 * time.Minute,
		Jitter: true,
	}

	for numCompletedJobs < numJobsToComplete {
		result := <-resultsChan
		if result.Err != nil {
			if isThrottlingError(result.Err) {
				t := b.Duration()
				if debug {
					fmt.Printf("\nbackoff triggered for %s,", result.Arn)
					fmt.Printf("\nwaiting for %v\n", t)
				}

				time.Sleep(t)
				jobs.Push(Job{Arn: result.Arn})

			} else if isStopworthyError(result.Err) {
				fmt.Printf("\nEncountered stopworthy error %v\nStopping run.\n", result.Err)
				for i := 0; i < parallel; i++ {
					quitChan <- true
				}

				return

			} else {
				failedJobs = append(failedJobs, result)
				numJobsToComplete--
			}

		} else {
			b.Reset()
			numCompletedJobs++
		}

		fmt.Printf("\r%d deregistered task definitions, %d errored", numCompletedJobs, len(failedJobs))

		if jobs.Len() > 0 {
			jobsChan <- jobs.Pop().(Job)
		}
	}

	if len(failedJobs) > 0 {
		fmt.Printf("\n%d task definitions errored.\n", len(failedJobs))
		if verbose {
			for _, result := range failedJobs {
				fmt.Printf("%s: %v\n", result.Arn, result.Err)
			}
		}
	}
}

func isThrottlingError(err error) bool {
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

func isStopworthyError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		code := awsErr.Code()

		if code == "ExpiredTokenException" {
			return true
		}
	}

	return false
}
