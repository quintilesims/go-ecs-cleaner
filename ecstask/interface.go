package ecstask

import "github.com/aws/aws-sdk-go/service/ecs"

type EcsSvc interface {
	DescribeServices(arg0 *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
	DeregisterTaskDefinition(*ecs.DeregisterTaskDefinitionInput) (*ecs.DeregisterTaskDefinitionOutput, error)
	ListClusters(*ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
	ListTaskDefinitions(*ecs.ListTaskDefinitionsInput) (*ecs.ListTaskDefinitionsOutput, error)
	ListServices(*ecs.ListServicesInput) (*ecs.ListServicesOutput, error)
}
