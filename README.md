# go-ecs-cleaner

A Go tool for cleaning up ECS resources in your AWS account.
CLI built using the [`cobra`](https://github.com/spf13/cobra) library.

## Installation

Download a binary appropriate for your OS from the [releases](https://github.com/quintilesims/go-ecs-cleaner/releases).

To build from source yourself, clone the repo, then build and use a binary, or run `main.go` directly:

- `go build && ./go-ecs-cleaner ecs-task`
- `go run main.go ecs-task`

## Usage

At present, `go-ecs-cleaner` isn't built to configure its own connection to AWS beyond specifying a region.
You'll need to use the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-welcome.html) - specifically, the `aws configure` command - to configure your credentials and connect to an account.

Once your environment is configured, you can run this tool!

`go-ecs-cleaner ecs-task --apply --region us-west-2`

Use the `-h, --help` flag to learn more about the tool's abilities.
