package ecsclient

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/jpillora/backoff"
	"github.com/quintilesims/go-ecs-cleaner/mocks"
)

func setupTest(t *testing.T) (*gomock.Controller, *ECSClient, *mocks.MockECSAPI) {
	ctrl := gomock.NewController(t)

	e := NewECSClient()
	e.Flags.Quiet = true

	svc := mocks.NewMockECSAPI(ctrl)
	e.Svc = svc

	return ctrl, e, svc
}

func Test_CollectClusters(t *testing.T) {
	ctrl, e, svc := setupTest(t)
	defer ctrl.Finish()

	expected := []string{"arn0", "arn1", "arn2"}

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

	result, err := e.CollectClusters()
	if err != nil {
		t.Error(err)
	}

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}
}

func Test_CollectServices(t *testing.T) {
	ctrl, e, svc := setupTest(t)
	defer ctrl.Finish()

	clusterARNs := []string{"cluster1", "cluster0"}

	expected := map[string][]string{
		"cluster0": []string{"service0", "service1"},
		"cluster1": []string{"service2"},
	}

	// paginated result
	svc.EXPECT().
		ListServices(&ecs.ListServicesInput{
			Cluster:   aws.String("cluster0"),
			NextToken: nil,
		}).
		Return(&ecs.ListServicesOutput{
			ServiceArns: []*string{aws.String("service0")},
			NextToken:   aws.String("a"),
		}, nil)

	svc.EXPECT().
		ListServices(&ecs.ListServicesInput{
			Cluster:   aws.String("cluster0"),
			NextToken: aws.String("a"),
		}).
		Return(&ecs.ListServicesOutput{
			ServiceArns: []*string{aws.String("service1")},
			NextToken:   nil,
		}, nil)

	// unpaginated result
	svc.EXPECT().
		ListServices(&ecs.ListServicesInput{
			Cluster:   aws.String("cluster1"),
			NextToken: nil,
		}).
		Return(&ecs.ListServicesOutput{
			ServiceArns: []*string{aws.String("service2")},
			NextToken:   nil,
		}, nil)

	result, err := e.CollectServices(clusterARNs)
	if err != nil {
		t.Error(err)
	}

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}
}

func Test_CollectTaskDefinitions(t *testing.T) {
	ctrl, e, svc := setupTest(t)
	defer ctrl.Finish()

	expected := []string{"arn0", "arn1", "arn2"}

	svc.EXPECT().
		ListTaskDefinitions(&ecs.ListTaskDefinitionsInput{}).
		Return(&ecs.ListTaskDefinitionsOutput{
			TaskDefinitionArns: []*string{aws.String("arn0"), aws.String("arn1")},
			NextToken:          aws.String("a"),
		}, nil)

	svc.EXPECT().
		ListTaskDefinitions(&ecs.ListTaskDefinitionsInput{
			NextToken: aws.String("a"),
		}).
		Return(&ecs.ListTaskDefinitionsOutput{
			TaskDefinitionArns: []*string{aws.String("arn2")},
			NextToken:          nil,
		}, nil)

	result, err := e.CollectTaskDefinitions()
	if err != nil {
		t.Error(err)
	}

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}
}

func Test_DescribeServices(t *testing.T) {

}

func Test_DeregisterTaskDefinitions_SunnyDay(t *testing.T) {
	ctrl, e, svc := setupTest(t)
	defer ctrl.Finish()

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

func Test_DeregisterTaskDefinitions_StopworthyError(t *testing.T) {
	ctrl, e, svc := setupTest(t)
	defer ctrl.Finish()

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

func Test_DeregisterTaskDefinitions_ThrottlingError(t *testing.T) {
	ctrl, e, svc := setupTest(t)
	defer ctrl.Finish()

	b := backoff.Backoff{
		Min:    1 * time.Millisecond,
		Max:    2 * time.Millisecond,
		Jitter: false,
	}

	e.Backoff = &b

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

func Test_FilterTaskDefinitions(t *testing.T) {

}

func Test_listClusters(t *testing.T) {

}

func Test_listServices(t *testing.T) {

}

func Test_listTaskDefinitions(t *testing.T) {

}

func Test_describeServices(t *testing.T) {

}

func Test_isExpiredTokenError(t *testing.T) {

}

func Test_isStopworthyError(t *testing.T) {

}

func Test_isThrottlingError(t *testing.T) {

}
