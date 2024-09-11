GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SHELL=/bin/bash

.PHONY: deps
deps:
	go mod vendor

.PHONY: build
build: $(GOFILES)
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags="-w -s"

.PHONY: docker-build
docker-build:
	docker build -f Dockerfile \
 		 -t supercronic:latest .

.PHONY: unit
unit:
	go test -v -race $$(go list ./... | grep -v /vendor/)
	go vet $$(go list ./... | grep -v /vendor/)

.PHONY: integration
integration: build
	bats integration

.PHONY: test
test: unit integration
	true

.PHONY: vulncheck
vulncheck: build
	govulncheck ./...

.PHONY: fmt
fmt:
	gofmt -l -w ${GOFILES_NOVENDOR}

.PHONY: release
release:
	./build.sh

.DEFAULT_GOAL := test
