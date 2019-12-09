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

## Docker

This repo publishes an image to DockerHub at [`quintilesims/go-ecs-cleaner`](https://hub.docker.com/r/quintilesims/go-ecs-cleaner), so you could pull it from there as well.
Running this image runs the tool's `ecs-task` command.

The Docker container takes its parameters as environment variables - yes, even the flags.
Here's an example:

```
docker run \
    -e FLAGS="-d -a" \
    -e AWS_REGION="us-west-2" \
    -e AWS_ACCESS_KEY="REDACTED" \
    -e AWS_SECRET_ACCESS_KEY="REDACTED" \
    go-ecs-cleaner:latest
```

## Helm

This repo contains a Helm chart that will deploy the `go-ecs-cleaner` tool into a Kubernetes cluster and run its `ecs-task` command.
The chart depends on configuration from two sources: secrets and user-specified values.

### Secrets

AWS Connection information is read from `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` keys stored in a Kubernetes secret.
Before you can install this chart, you must create this secret and populate it.
You can do so with `kubectl`:

```
kubectl create secret generic ecs-task-cleaner-secrets \
    --from-literal AWS_ACCESS_KEY_ID=your_value_here \
    --from-literal AWS_SECRET_ACCESS_KEY=your_value_here
```

If you would like to name your secret something other than `ecs-task-cleaner-secrets` you may do so as long as you provide the name of your custom secret when you install the Helm chart.

### User-Defined Values

`AWS_REGION` and `FLAGS` should be specified in the `values.yaml` file, or passed as `helm install` arguments.

- `values.yaml`:

```
env:
  AWS_REGION: "us-west-2"
  FLAGS: "--apply --debug"
```

- `helm install`:

```
helm install \
    --set env.AWS_REGION="us-west-2" \
    --set env.FLAGS="--apply --debug" \
    --values PATH_TO_POPULATED_VALUES.YAML \
    ecs-task-cleaner ./ecs-task-cleaner
```

The name of the Kubernetes secret is `ecs-task-cleaner-secrets` by default, but you can change this by specifying `kubernetesSecretName:` in your `values.yaml` file or specifying `--set kubernetesSecretName=` in the `helm install` command.
