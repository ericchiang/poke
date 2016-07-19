PROJ="poke"
ORG_PATH="github.com/ericchiang"
REPO_PATH="$(ORG_PATH)/$(PROJ)"

$(shell mkdir -p bin thirdparty)

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


build: bin/rolo bin/roloctl

bin/rolo: FORCE
	@go install $(REPO_PATH)/cmd/poke

bin/roloctl: FORCE
	@go install $(REPO_PATH)/cmd/pokectl

test:
	@go test $(shell go list ./... | grep -v '/vendor/')

testrace:
	@go test --race $(shell go list ./... | grep -v '/vendor/')

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
