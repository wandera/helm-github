SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: test build

.PHONY: build
build:
	go build -o bin/helmgithub

.PHONY: test
test:
	go test ./... -v
