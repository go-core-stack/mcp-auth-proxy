# Copyright © 2025 Prabhjot Singh Sethi, All Rights reserved
# Author: Prabhjot Singh Sethi <prabhjot.sethi@gmail.com>

include config.mk

# image name of the mcp-auth-proxy container
IMG:= $(REPO)/mcp-auth-proxy:$(VERSION)

# root directory for the build
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

.PHONY: all build go-format go-vet go-lint push-images

.DEFAULT_GOAL := build

go-format:
	go fmt ./...

go-vet:
	go vet ./...

go-lint:
	golangci-lint run

build: go-format go-vet go-lint
	docker build --build-arg GIT_TOKEN="${GIT_TOKEN}" \
		-t ${IMG} .

push-images: build
	docker push ${IMG}
