WORKDIR := $(shell pwd)
REGISTRY := eu.gcr.io/sap-se-gcp-scp-k8s/remedy-controller
IMAGE_NAME := remedy-controller
IMAGE_TAG := $(shell cat VERSION)
LD_FLAGS := "-s -w -X github.wdf.sap.corp/kubernetes/remedy-controller/pkg/version.gitVersion=$(IMAGE_TAG) -X github.wdf.sap.corp/kubernetes/remedy-controller/pkg/version.gitCommit=$(shell git rev-parse --verify HEAD) -X github.wdf.sap.corp/kubernetes/remedy-controller/pkg/version.buildDate=$(shell date --rfc-3339=seconds | sed 's/ /T/')"

.PHONY: build
build:
	@mkdir -p $(WORKDIR)/bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
		-mod=vendor \
		-ldflags $(LD_FLAGS) \
		-o $(WORKDIR)/bin/remedy-controller-linux-amd64 \
		$(WORKDIR)/main.go
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
		-ldflags $(LD_FLAGS) \
		-o $(WORKDIR)/bin/remedy-controller-darwin-amd64 \
		$(WORKDIR)/main.go
	@

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod vendor
	@GO111MODULE=on go mod tidy

.PHONY: docker-build
docker-build:
	@docker build -t $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) $(WORKDIR)