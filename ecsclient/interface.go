package ecsclient

import "github.com/aws/aws-sdk-go/service/ecs"

type ECSSvc interface {
	DescribeServices(*ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
	DeregisterTaskDefinition(*ecs.DeregisterTaskDefinitionInput) (*ecs.DeregisterTaskDefinitionOutput, error)
	ListClusters(*ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
	ListServices(*ecs.ListServicesInput) (*ecs.ListServicesOutput, error)
	ListTaskDefinitions(*ecs.ListTaskDefinitionsInput) (*ecs.ListTaskDefinitionsOutput, error)
}

type ECSClientIface interface {
	CleanupTaskDefinitions() error
	CollectClusters() ([]string, error)
	CollectServices([]string) (map[string][]string, error)
	CollectTaskDefinitions([]ecs.Service) ([]string, error)
	ConfigureSession() error
	DeregisterTaskDefinitions([]string) error
	DescribeServices(map[string][]string) ([]ecs.Service, error)
	FilterTaskDefinitions([]string) ([]string, error)
}
