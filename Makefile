WORKDIR := $(shell pwd)

.PHONY: build
build:
	@mkdir -p $(WORKDIR)/bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
		-mod=vendor \
		-o $(WORKDIR)/bin/remedy-controller-linux-amd64 \
		$(WORKDIR)/main.go
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
		-o $(WORKDIR)/bin/remedy-controller-darwin-amd64 \
		$(WORKDIR)/main.go

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod vendor
	@GO111MODULE=on go mod tidy