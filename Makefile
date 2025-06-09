GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SHELL=/bin/bash
VERSION=$(shell git describe --tags --always --dirty)

.PHONY: deps
deps:
	go mod vendor

.PHONY: build
build: $(GOFILES)
	go build -ldflags "-X main.Version=${VERSION}"

.PHONY: unit
unit:
	go test -v -race $$(go list ./... | grep -v /vendor/)
	go vet $$(go list ./... | grep -v /vendor/)

.PHONY: integration
integration: VERSION=v1337
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

.DEFAULT_GOAL := test
