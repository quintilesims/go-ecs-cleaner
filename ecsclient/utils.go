package ecsclient

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
)

// Job carries information through a Job channel.
type Job struct {
	Arn string
}

// Result carries information through a Result channel.
type Result struct {
	Arn string
	Err error
}

// Receives dispatched jobs and deregisters the task definitions contained
// therein. Intended to be run as a goroutine.
func doDeregistrationJobs(svc ECSSvc, wg *sync.WaitGroup, jobsChan <-chan Job, resultsChan chan<- Result, quitChan <-chan bool) {
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

// Wrangles task definition ARNs into a stack of Jobs and dispatches them to worker
// goroutines via channels. Intended to be run as a goroutine.
func dispatchDeregistrationJobs(wg *sync.WaitGroup, arns []string, parallel int, jobsChan chan Job, resultsChan chan Result, quitChan chan<- bool, errChan chan<- error, verbose, debug bool) {
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
				if debug {
					fmt.Printf("\nDispatcher encountered stopworthy error %v\nStopping run.\n", result.Err)
				}

				for i := 0; i < parallel; i++ {
					quitChan <- true
				}

				errChan <- result.Err

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

	errChan <- nil
}

// Checks whether a given error is something we would consider to be a throttling
// error.
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

// Checks whether a given error is something we would consider stopping the execution
// of the worker pool handling task definition deregistration jobs.
func isStopworthyError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		code := awsErr.Code()

		if code == "ExpiredTokenException" {
			return true
		}
	}

	return false
}

// Given two lists of strings, ensures that list B contains none of the items in
// list A. If an A-item is in list B, it is removed from list B. If an A-item is
// not in list B, it is not added to list B. The remaining list B items are returned.
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
