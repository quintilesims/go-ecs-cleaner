mocks:
	mockgen \
		-destination mocks/mock_ecs.go \
		-package mocks "github.com/aws/aws-sdk-go/service/ecs/ecsiface" ECSAPI

.PHONY: mocks