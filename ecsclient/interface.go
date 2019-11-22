package ecsclient

import "github.com/aws/aws-sdk-go/service/ecs"

// ECSSvc defines the methods that an object must have in order to be used as the `Svc`
// object in the ECSClient. The AWS `ecs.ECS` object satisfies this interface.
type ECSSvc interface {
	DescribeServices(*ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
	DeregisterTaskDefinition(*ecs.DeregisterTaskDefinitionInput) (*ecs.DeregisterTaskDefinitionOutput, error)
	ListClusters(*ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
	ListServices(*ecs.ListServicesInput) (*ecs.ListServicesOutput, error)
	ListTaskDefinitions(*ecs.ListTaskDefinitionsInput) (*ecs.ListTaskDefinitionsOutput, error)
}
