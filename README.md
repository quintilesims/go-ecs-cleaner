# go-ecs-cleaner

A Go tool for cleaning up ECS resources in your AWS account.
CLI built using the [`cobra`](https://github.com/spf13/cobra) library.

## Installation

Download a binary appropriate for your OS from the [releases](https://github.com/quintilesims/go-ecs-cleaner/releases).

To build from source yourself, clone the repo, then build and use a binary, or run `main.go` directly:

- `go build && ./go-ecs-cleaner ecs-task`
- `go run main.go ecs-task`

## Usage

The `go-ecs-cleaner` tool takes AWS configuration parameters from these environment variables:

- AWS_ACCESS_KEY
- AWS_SECRET_ACCESS_KEY
- AWS_REGION

Use the `-h, --help` flag to learn more about the tool's abilities.
