# Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

NAME                        := remedy-controller
APPLIER_NAME                := remedy-applier
REGISTRY                    := eu.gcr.io/gardener-project/gardener/remedy-controller
IMAGE_PREFIX                := $(REGISTRY)
REPO_ROOT                   := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
VERSION                     := $(shell cat "$(REPO_ROOT)/VERSION")
LD_FLAGS                    := "-w -X github.com/gardener/$(NAME)/pkg/version.Version=$(VERSION) -X github.com/gardener/$(NAME)/pkg/version.GitCommit=$(shell git rev-parse --verify HEAD) -X github.com/gardener/$(NAME)/pkg/version.BuildDate=$(shell date --rfc-3339=seconds | sed 's/ /T/')"
LEADER_ELECTION             := false

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: start-azure
start-azure:
	@LEADER_ELECTION_NAMESPACE=garden GO111MODULE=on go run \
		-mod=vendor \
		-ldflags $(LD_FLAGS) \
		./cmd/$(NAME)-azure \
		--config-file=./example/00-config.yaml \
		--leader-election=$(LEADER_ELECTION) \
		--namespace=kube-system \
		--metrics-bind-address=":6000" \
		--target-metrics-bind-address=":6001" \
		--infrastructure-config=dev/credentials.yaml

.PHONY: start-applier-azure
start-applier-azure:
	@GO111MODULE=on go run \
		-mod=vendor \
		-ldflags $(LD_FLAGS) \
		./cmd/$(APPLIER_NAME)-azure \
		--kubeconfig=$(KUBECONFIG) \
		--infrastructure-config=dev/credentials.yaml

.PHONY: start-failedvm-simulator-azure
VM_NAME = ""
start-failedvm-simulator-azure:
	@GO111MODULE=on go run \
		-mod=vendor \
		-ldflags $(LD_FLAGS) \
		./cmd/failedvm-simulator-azure \
		$(VM_NAME)

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@LD_FLAGS="-w -X github.com/gardener/$(NAME)/pkg/version.Version=$(VERSION) -X github.com/gardener/$(NAME)/pkg/version.GitCommit=$(shell git rev-parse --verify HEAD) -X github.com/gardener/$(NAME)/pkg/version.BuildDate=$(shell date --rfc-3339=seconds | sed 's/ /T/')" \
	$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/install.sh ./...

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-images
docker-images:
	@docker build -t $(IMAGE_PREFIX)/$(NAME)-azure:$(VERSION) -t $(IMAGE_PREFIX)/$(NAME)-azure:latest -f Dockerfile -m 6g --target $(NAME)-azure .

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: install-requirements
install-requirements: # needs sudo permissions
	@go install -mod=vendor $(REPO_ROOT)/vendor/github.com/ahmetb/gen-crd-api-reference-docs
	@go install -mod=vendor $(REPO_ROOT)/vendor/github.com/golang/mock/mockgen
	@go install -mod=vendor $(REPO_ROOT)/vendor/github.com/onsi/ginkgo/v2/ginkgo
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/install-requirements.sh
	@echo "install newer version of golangci-lint"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1 

	@python3 -m venv $(REPO_ROOT)/.env
	@. $(REPO_ROOT)/.env/bin/activate && pip3 install --upgrade pip && pip3 install -r $(REPO_ROOT)/test/requirements.txt

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod vendor
	@GO111MODULE=on go mod tidy
	@chmod +x $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/*
	@chmod +x $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/.ci/*
	@$(REPO_ROOT)/hack/update-github-templates.sh

.PHONY: clean
clean:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/clean.sh ./cmd/... ./pkg/... ./test/...

.PHONY: check-generate
check-generate:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check-generate.sh $(REPO_ROOT)

.PHONY: check
check:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./pkg/...
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check-charts.sh ./charts
	@. $(REPO_ROOT)/.env/bin/activate && flake8 $(REPO_ROOT)/test

.PHONY: generate
generate:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/generate.sh ./cmd/... ./pkg/...

.PHONY: format
format:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/format.sh ./cmd ./pkg

.PHONY: test
test:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test.sh ./cmd/... ./pkg/...

.PHONY: test-cov
test-cov:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test-cover.sh ./cmd/... ./pkg/...

.PHONY: test-clean
test-clean:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test-cover-clean.sh

.PHONY: verify
verify: check format test

.PHONY: verify-extended
verify-extended: install-requirements check-generate check format test-cov test-clean

.PHONY: pubip-remedy-test
pubip-remedy-test:
	@. $(REPO_ROOT)/.env/bin/activate && python3 $(REPO_ROOT)/test/pubip_remedy_test.py --credentials-path "$(REPO_ROOT)/dev/credentials.yaml"

.PHONY: failed-vm-test
WORKER_GROUP = ""
failed-vm-test:
	test -n $(WORKER_GROUP) # WORKER_GROUP must be given
	@. $(REPO_ROOT)/.env/bin/activate && python3 $(REPO_ROOT)/test/failed_vm_test.py --credentials-path "$(REPO_ROOT)/dev/credentials.yaml" --fail-worker-group-name $(WORKER_GROUP)

.PHONY: test-cleanup
test-cleanup:
	@. $(REPO_ROOT)/.env/bin/activate && python3 $(REPO_ROOT)/test/cleanup.py --credentials-path "$(REPO_ROOT)/dev/credentials.yaml"

