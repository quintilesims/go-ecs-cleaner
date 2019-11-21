mocks:
	mockgen -destination mock_ecsiface/mock_ecs.go "github.com/aws/aws-sdk-go/service/ecs/ecsiface" ECSAPI