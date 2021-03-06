PROJ="poke"
ORG_PATH="github.com/ericchiang"
REPO_PATH="$(ORG_PATH)/$(PROJ)"

export GOBIN=$(PWD)/bin
export GO15VENDOREXPERIMENT=1

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

COMMIT=$(shell git rev-parse HEAD)

# check if the current commit has a matching tag
TAG=$(shell git describe --exact-match --abbrev=0 --tags $(COMMIT) 2> /dev/null || true)

ifeq ($(TAG),)
	VERSION=$(TAG)
else
	VERSION=$(COMMIT)
endif


build: bin/poke bin/pokectl

bin/poke: FORCE
	@go install $(REPO_PATH)/cmd/poke

bin/pokectl: FORCE
	@go install $(REPO_PATH)/cmd/pokectl

test:
	@go test -v $(shell go list ./... | grep -v '/vendor/')

testrace:
	@go test -v --race $(shell go list ./... | grep -v '/vendor/')

vet:
	@go vet $(shell go list ./... | grep -v '/vendor/')

fmt:
	@go fmt $(shell go list ./... | grep -v '/vendor/')

lint:
	@for package in $(shell go list ./... | grep -v '/vendor/'); do \
      golint $$package; \
	done

testall: testrace vet fmt lint

FORCE:

.PHONY: test testrace vet fmt lint testall
