.PHONY: test bench build install version

PACKAGES_TO_TEST := $(shell go list ./... | grep -v -E '/(benchmarks|examples)')
VERSION := $(shell cat VERSION)
LDFLAGS := -ldflags "-X github.com/seuros/gopher-cypher/src/internal/boltutil.LibraryVersion=$(VERSION)"

test:
	go test $(PACKAGES_TO_TEST)

bench:
	cd benchmarks && go test -bench=.

build:
	go build $(LDFLAGS) -o cyq ./cmd/cyq/

install:
	go install $(LDFLAGS) ./cmd/cyq/

version:
	@echo $(VERSION)
