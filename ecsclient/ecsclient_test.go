package ecsclient

import (
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/jpillora/backoff"
	"github.com/quintilesims/go-ecs-cleaner/mocks"
)

func setup(t *testing.T) (*gomock.Controller, *ECSClient, *mocks.MockECSAPI) {
	ctrl := gomock.NewController(t)

	e := NewECSClient()
	e.Flags.Quiet = true

	svc := mocks.NewMockECSAPI(ctrl)
	e.Svc = svc

	return ctrl, e, svc
}

func Test_CollectClusters(t *testing.T) {
	ctrl, e, svc := setup(t)
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
	ctrl, e, svc := setup(t)
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
	ctrl, e, svc := setup(t)
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

func Test_DeregisterTaskDefinitions_SunnyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
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
	ctrl, e, svc := setup(t)
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
	ctrl, e, svc := setup(t)
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

func Test_DescribeServices(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	serviceARNsByClusterARN := map[string][]string{
		"arn0": []string{"service0", "service1"},
		"arn1": []string{"service2"},
	}

	expected := []ecs.Service{
		ecs.Service{ServiceArn: aws.String("service0")},
		ecs.Service{ServiceArn: aws.String("service1")},
		ecs.Service{ServiceArn: aws.String("service2")},
	}

	svc.EXPECT().
		DescribeServices(&ecs.DescribeServicesInput{
			Cluster:  aws.String("arn0"),
			Services: []*string{aws.String("service0"), aws.String("service1")},
		}).
		Return(&ecs.DescribeServicesOutput{
			Services: []*ecs.Service{
				&ecs.Service{ServiceArn: aws.String("service0")},
				&ecs.Service{ServiceArn: aws.String("service1")},
			},
		}, nil)

	svc.EXPECT().
		DescribeServices(&ecs.DescribeServicesInput{
			Cluster:  aws.String("arn1"),
			Services: []*string{aws.String("service2")},
		}).
		Return(&ecs.DescribeServicesOutput{
			Services: []*ecs.Service{
				&ecs.Service{ServiceArn: aws.String("service2")},
			},
		}, nil)

	result, err := e.DescribeServices(serviceARNsByClusterARN)
	if err != nil {
		t.Error(err)
	}

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}
}

func Test_FilterTaskDefinitions(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	e.Flags.Cutoff = 2

	allARNs := []string{
		// service in use, cutoff=2;
		// should filter out aws-blather:family0:3, aws-blather:family0:2, aws-blather:family0:1
		"aws-blather:family0:0", "aws-blather:family0:1", "aws-blather:family0:2", "aws-blather:family0:3",

		// service in use, cutoff=2;
		// should filter out all three of these
		"aws-blather:family1:0", "aws-blather:family1:1", "aws-blather:family1:2",

		// service in use, cutoff=2; should filter out both of these
		"aws-blather:family2:0", "aws-blather:family2:1",

		// service in use, cutoff=2; should filter out this one
		"aws-blather:family3:0",

		// service not in use; should filter out neither of these
		"aws-blather:family4:0", "aws-blather:family4:1",
	}

	services := []ecs.Service{
		ecs.Service{TaskDefinition: aws.String("aws-blather:family0:3")},
		ecs.Service{TaskDefinition: aws.String("aws-blather:family1:2")},
		ecs.Service{TaskDefinition: aws.String("aws-blather:family2:1")},
		ecs.Service{TaskDefinition: aws.String("aws-blather:family3:0")},
	}

	expected := []string{"aws-blather:family0:0", "aws-blather:family4:0", "aws-blather:family4:1"}

	svc.EXPECT().
		ListTaskDefinitions(&ecs.ListTaskDefinitionsInput{
			FamilyPrefix: aws.String("family0"),
			Sort:         aws.String("DESC"),
		}).
		Return(&ecs.ListTaskDefinitionsOutput{
			TaskDefinitionArns: []*string{
				aws.String("aws-blather:family0:3"),
				aws.String("aws-blather:family0:2"),
				aws.String("aws-blather:family0:1"),
				aws.String("aws-blather:family0:0"),
			},
			NextToken: nil,
		}, nil)

	svc.EXPECT().
		ListTaskDefinitions(&ecs.ListTaskDefinitionsInput{
			FamilyPrefix: aws.String("family1"),
			Sort:         aws.String("DESC"),
		}).
		Return(&ecs.ListTaskDefinitionsOutput{
			TaskDefinitionArns: []*string{
				aws.String("aws-blather:family1:2"),
				aws.String("aws-blather:family1:1"),
				aws.String("aws-blather:family1:0"),
			},
			NextToken: nil,
		}, nil)

	svc.EXPECT().
		ListTaskDefinitions(&ecs.ListTaskDefinitionsInput{
			FamilyPrefix: aws.String("family2"),
			Sort:         aws.String("DESC"),
		}).
		Return(&ecs.ListTaskDefinitionsOutput{
			TaskDefinitionArns: []*string{
				aws.String("aws-blather:family2:1"),
				aws.String("aws-blather:family2:0"),
			},
			NextToken: nil,
		}, nil)

	svc.EXPECT().
		ListTaskDefinitions(&ecs.ListTaskDefinitionsInput{
			FamilyPrefix: aws.String("family3"),
			Sort:         aws.String("DESC"),
		}).
		Return(&ecs.ListTaskDefinitionsOutput{
			TaskDefinitionArns: []*string{
				aws.String("aws-blather:family3:0"),
			},
			NextToken: nil,
		}, nil)

	result, err := e.FilterTaskDefinitions(allARNs, services)
	if err != nil {
		t.Error(err)
	}

	sort.Strings(expected)
	sort.Strings(result)

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}
}

func Test_listClusters_SunnyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	nextToken := "a"

	expected := []string{"cluster0", "cluster1"}
	expectedToken := "b"

	svc.EXPECT().
		ListClusters(&ecs.ListClustersInput{
			NextToken: &nextToken,
		}).
		Return(&ecs.ListClustersOutput{
			ClusterArns: []*string{aws.String("cluster0"), aws.String("cluster1")},
			NextToken:   &expectedToken,
		}, nil)

	result, token, err := e.listClusters(&nextToken)

	if err != nil {
		t.Error(err)
	}

	if token != &expectedToken {
		t.Errorf("Expected token %v, got %v\n", expectedToken, token)
	}

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}
}

func Test_listClusters_RainyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	expectedError := errors.New("IntentionalException")

	svc.EXPECT().
		ListClusters(gomock.Any()).
		Return(nil, expectedError)

	result, token, err := e.listClusters(nil)

	if equal := reflect.DeepEqual([]string{}, result); !equal {
		t.Errorf("Expected %v, got %v\n", []string{}, result)
	}

	if token != nil {
		t.Errorf("Expected token %v, got %v\n", nil, token)
	}

	if err == nil {
		t.Errorf("Expected error %v, got %v\n", expectedError, err)
	}
}

func Test_listServices_SunnyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	givenToken := "a"

	expected := []string{"service0"}
	expectedToken := "b"

	svc.EXPECT().
		ListServices(&ecs.ListServicesInput{
			Cluster:   aws.String("cluster0"),
			NextToken: &givenToken,
		}).
		Return(&ecs.ListServicesOutput{
			ServiceArns: []*string{aws.String("service0")},
			NextToken:   &expectedToken,
		}, nil)

	result, token, err := e.listServices("cluster0", &givenToken)

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}

	if token != &expectedToken {
		t.Errorf("Expected token %v, got %v\n", expectedToken, token)
	}

	if err != nil {
		t.Error(err)
	}
}

func Test_listServices_RainyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	expectedError := errors.New("IntentionalException")

	svc.EXPECT().
		ListServices(gomock.Any()).
		Return(nil, expectedError)

	result, token, err := e.listServices("", nil)

	if equal := reflect.DeepEqual([]string{}, result); !equal {
		t.Errorf("Expected %v, got %v\n", []string{}, result)
	}

	if token != nil {
		t.Errorf("Expected token %v, got %v\n", nil, token)
	}

	if err == nil {
		t.Errorf("Expected error %v, got %v\n", expectedError, err)
	}
}

func Test_listTaskDefinitions_SunnyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	givenToken := "a"

	expected := []string{"taskdef0"}
	expectedToken := "b"

	svc.EXPECT().
		ListTaskDefinitions(&ecs.ListTaskDefinitionsInput{
			FamilyPrefix: aws.String("family0"),
			Sort:         aws.String("DESC"),
			NextToken:    &givenToken,
		}).
		Return(&ecs.ListTaskDefinitionsOutput{
			TaskDefinitionArns: []*string{aws.String("taskdef0")},
			NextToken:          &expectedToken,
		}, nil)

	result, token, err := e.listTaskDefinitions("family0", "DESC", &givenToken)

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}

	if token != &expectedToken {
		t.Errorf("Expected token %v, got %v\n", expectedToken, token)
	}

	if err != nil {
		t.Error(err)
	}
}

func Test_listTaskDefinitions_RainyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	expectedError := errors.New("IntentionalException")

	svc.EXPECT().
		ListTaskDefinitions(gomock.Any()).
		Return(nil, expectedError)

	result, token, err := e.listTaskDefinitions("", "", nil)

	if equal := reflect.DeepEqual([]string{}, result); !equal {
		t.Errorf("Expected %v, got %v\n", []string{}, result)
	}

	if token != nil {
		t.Errorf("Expected token %v, got %v\n", nil, token)
	}

	if err == nil {
		t.Errorf("Expected error %v, got %v\n", expectedError, err)
	}
}

func Test_describeServices_SunnyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	expected := []ecs.Service{
		ecs.Service{ServiceArn: aws.String("service0")},
		ecs.Service{ServiceArn: aws.String("service1")},
	}

	svc.EXPECT().
		DescribeServices(&ecs.DescribeServicesInput{
			Cluster:  aws.String("cluster0"),
			Services: []*string{aws.String("service0"), aws.String("service1")},
		}).
		Return(
			&ecs.DescribeServicesOutput{
				Services: []*ecs.Service{
					&ecs.Service{ServiceArn: aws.String("service0")},
					&ecs.Service{ServiceArn: aws.String("service1")},
				},
			}, nil)

	result, err := e.describeServices("cluster0", []string{"service0", "service1"})

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}

	if err != nil {
		t.Error(err)
	}
}

func Test_describeServices_RainyDay(t *testing.T) {
	ctrl, e, svc := setup(t)
	defer ctrl.Finish()

	expected := []ecs.Service{}
	expectedError := errors.New("IntentionalException")

	svc.EXPECT().
		DescribeServices(gomock.Any()).
		Return(nil, expectedError)

	result, err := e.describeServices("", []string{})

	if equal := reflect.DeepEqual(expected, result); !equal {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}

	if err != expectedError {
		t.Errorf("Expected error %v, got %v\n", expectedError, err)
	}
}

func Test_isExpiredTokenError(t *testing.T) {
	testCases := map[awserr.Error]bool{
		awserr.New("ClientException", "too many concurrent attempts", errors.New("")): false,
		awserr.New("ExpiredTokenException", "", errors.New("")):                       true,
		awserr.New("Throttling", "", errors.New("")):                                  false,
		awserr.New("ThrottlingException", "", errors.New("")):                         false,
		awserr.New("", "", errors.New("")):                                            false,
	}

	for testCase, expected := range testCases {
		ctrl, e, _ := setup(t)
		defer ctrl.Finish()

		result := e.isExpiredTokenError(testCase)

		if expected != result {
			t.Errorf("TestCase '%v': expected %t, received %t\n", testCase, expected, result)
		}
	}
}

func Test_isStopworthyError(t *testing.T) {
	testCases := map[awserr.Error]bool{
		awserr.New("ClientException", "too many concurrent attempts", errors.New("")): false,
		awserr.New("ExpiredTokenException", "", errors.New("")):                       false,
		awserr.New("Throttling", "", errors.New("")):                                  false,
		awserr.New("ThrottlingException", "", errors.New("")):                         false,
		awserr.New("", "", errors.New("")):                                            true,
	}

	for testCase, expected := range testCases {
		ctrl, e, _ := setup(t)
		defer ctrl.Finish()

		result := e.isStopworthyError(testCase)

		if expected != result {
			t.Errorf("TestCase '%v', expected %t, received %t\n", testCase, expected, result)
		}
	}
}

func Test_isThrottlingError(t *testing.T) {
	testCases := map[awserr.Error]bool{
		awserr.New("ClientException", "too many concurrent attempts", errors.New("")): true,
		awserr.New("ExpiredTokenException", "", errors.New("")):                       false,
		awserr.New("Throttling", "", errors.New("")):                                  true,
		awserr.New("ThrottlingException", "", errors.New("")):                         true,
		awserr.New("", "", errors.New("")):                                            false,
	}

	for testCase, expected := range testCases {
		ctrl, e, _ := setup(t)
		defer ctrl.Finish()

		result := e.isThrottlingError(testCase)

		if expected != result {
			t.Errorf("TestCase '%v', expected %t, received %t\n", testCase, expected, result)
		}
	}
}
