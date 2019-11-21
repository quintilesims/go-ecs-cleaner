package ecstask

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	gomock "github.com/golang/mock/gomock"
	"github.com/quintilesims/go-ecs-cleaner/mock_ecsiface"
)

func Test_deregisterTaskDefinitions_(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := mock_ecsiface.NewMockECSAPI(ctrl)
	taskDefinitionArns := []string{"a", "b", "c", "d", "e", "f", "g"}
	parallel := 1
	verbose, debug := false, false

	for _, arn := range taskDefinitionArns {
		svc.EXPECT().
			DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: aws.String(arn),
			}).
			Return(&ecs.DeregisterTaskDefinitionOutput{}, nil)
	}

	deregisterTaskDefinitions(svc, taskDefinitionArns, parallel, verbose, debug)
}
