SHELL := /bin/bash
DOCKER_REPO ?= quintilesims/go-ecs-cleaner

.EXPORT_ALL_VARIABLES:

mocks:
	mockgen \
		-destination mocks/mock_ecs.go \
		-package mocks "github.com/aws/aws-sdk-go/service/ecs/ecsiface" ECSAPI

check-vars:
	@ if [[ ! -v VERSION_TAG ]] ; then \
		echo "VERSION_TAG is not set!" ; \
		exit 1 ; \
	fi

go-build: check-vars
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "-s -X main.Version=${VERSION_TAG}" -a -o build/Linux/go-ecs-cleaner .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -X main.Version=${VERSION_TAG}" -a -o build/macOS/go-ecs-cleaner .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -X main.Version=${VERSION_TAG}" -a -o build/Windows/go-ecs-cleaner.exe .

docker-build: check-vars
	docker build -t ${DOCKER_REPO}:${VERSION_TAG} .

docker-push: check-vars
	docker push ${DOCKER_REPO}:${VERSION_TAG}

publish-docker-image: go-build docker-build docker-push

.PHONY: mocks go-build docker-build docker-push publish-docker-image