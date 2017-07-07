GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SHELL=/bin/bash

.PHONY: build
build: $(GOFILES)
	go build

.PHONY: test
test:
	go test $$(go list ./... | grep -v /vendor/)
	go vet $$(go list ./... | grep -v /vendor/)

.PHONY: fmt
fmt:
	gofmt -l -w ${GOFILES_NOVENDOR}
