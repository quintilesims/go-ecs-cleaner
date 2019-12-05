package ecsclient

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/jpillora/backoff"
	"github.com/quintilesims/go-ecs-cleaner/mocks"
)

func TestCollectClusters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	e := NewECSClient()
	e.Flags.Quiet = true

	svc := mocks.NewMockECSAPI(ctrl)
	e.Svc = svc

	svc.EXPECT().
		ListClusters(&ecs.ListClustersInput{
			NextToken: nil,
		}).
		Return(&ecs.ListClustersOutput{
			ClusterArns: []*string{aws.String("arn0"), aws.String("arn1")},
			NextToken:   aws.String("a"),
		}, nil)

	svc.EXPECT().
		ListClusters(&ecs.ListClustersInput{
			NextToken: aws.String("a"),
		}).
		Return(&ecs.ListClustersOutput{
			ClusterArns: []*string{aws.String("arn2")},
			NextToken:   nil,
		}, nil)

	expectedClusters := []string{"arn0", "arn1", "arn2"}
	clusters, err := e.CollectClusters()

	if err != nil {
		t.Error("error encountered, ", err)
	}

	for i := 0; i < len(expectedClusters); i++ {
		if clusters[i] != expectedClusters[i] {
			t.Errorf("clusters[%d] (%s) != expectedClusters[%d] (%s)", i, clusters[i], i, expectedClusters[i])
		}
	}
}

func TestDeregisterTaskDefinitions_SunnyDay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	e := NewECSClient()
	e.Flags.Quiet = true

	svc := mocks.NewMockECSAPI(ctrl)
	e.Svc = svc

	arns := []string{"arn0", "arn1", "arn2"}

	svc.EXPECT().
		DeregisterTaskDefinition(gomock.Any()).
		Return(&ecs.DeregisterTaskDefinitionOutput{}, nil).
		Times(3)

	err := e.DeregisterTaskDefinitions(arns)
	if err != nil {
		t.Error("error encountered: ", err)
	}
}

func TestDeregisterTaskDefinitions_StopworthyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	e := NewECSClient()
	e.Flags.Quiet = true

	svc := mocks.NewMockECSAPI(ctrl)
	e.Svc = svc

	arns := []string{"arn0"}

	awsErr := awserr.New("", "", errors.New(""))

	svc.EXPECT().
		DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String("arn0"),
		}).
		Return(
			&ecs.DeregisterTaskDefinitionOutput{},
			awsErr,
		)

	err := e.DeregisterTaskDefinitions(arns)
	if err != awsErr {
		t.Error("did not receive expected error")
	}
}

func TestDeregisterTaskDefinitions_ThrottlingError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	e := NewECSClient()
	e.Flags.Quiet = true

	b := backoff.Backoff{
		Min:    1 * time.Millisecond,
		Max:    2 * time.Millisecond,
		Jitter: false,
	}

	e.Backoff = &b

	svc := mocks.NewMockECSAPI(ctrl)
	e.Svc = svc

	arns := []string{"arn0"}

	// There are three different forms that what the ECSClient considers to be a throttling
	// error can take. Whenever the client encounters one, it waits according to its Backoff
	// controller and then retries the job. If each of these forms of errors are thrown once,
	// we can expect four calls to svc.DeregisterTaskDefinition().

	svc.EXPECT().
		DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String("arn0"),
		}).
		Return(
			&ecs.DeregisterTaskDefinitionOutput{},
			awserr.New("Throttling", "", errors.New("")),
		)

	svc.EXPECT().
		DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String("arn0"),
		}).
		Return(
			&ecs.DeregisterTaskDefinitionOutput{},
			awserr.New("ThrottlingException", "", errors.New("")),
		)

	svc.EXPECT().
		DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String("arn0"),
		}).
		Return(
			&ecs.DeregisterTaskDefinitionOutput{},
			awserr.New("ClientException", "too many concurrent attempts", errors.New("")),
		)

	svc.EXPECT().
		DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String("arn0"),
		}).
		Return(&ecs.DeregisterTaskDefinitionOutput{}, nil)

	err := e.DeregisterTaskDefinitions(arns)
	if err != nil {
		t.Error("error encountered: ", err)
	}
}
