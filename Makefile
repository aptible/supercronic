GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SHELL=/bin/bash

.PHONY: deps
deps:
	go mod vendor

.PHONY: build
build: $(GOFILES)
	go build

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

.PHONY: fmt
fmt:
	gofmt -l -w ${GOFILES_NOVENDOR}

.DEFAULT_GOAL := test
