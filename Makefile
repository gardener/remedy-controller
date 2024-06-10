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

GARDENER_HACK_DIR           := $(shell go list -m -f "{{.Dir}}" github.com/gardener/gardener)/hack
NAME                        := remedy-controller
APPLIER_NAME                := remedy-applier
REGISTRY                    := europe-docker.pkg.dev/gardener-project/public
IMAGE_PREFIX                := $(REGISTRY)/gardener/remedy-controller
REPO_ROOT                   := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
HACK_DIR                    := $(REPO_ROOT)/hack
VERSION                     := $(shell cat "$(REPO_ROOT)/VERSION")
LD_FLAGS                    := "-w -X github.com/gardener/$(NAME)/pkg/version.Version=$(VERSION) -X github.com/gardener/$(NAME)/pkg/version.GitCommit=$(shell git rev-parse --verify HEAD) -X github.com/gardener/$(NAME)/pkg/version.BuildDate=$(shell date --rfc-3339=seconds | sed 's/ /T/')"
LEADER_ELECTION             := false

#########################################
# Tools                                 #
#########################################

TOOLS_DIR := $(HACK_DIR)/tools
include $(GARDENER_HACK_DIR)/tools.mk
GOLANGCI_LINT_VERSION := v1.55.2

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: start-azure
start-azure:
	@LEADER_ELECTION_NAMESPACE=garden GO111MODULE=on go run \
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
		-ldflags $(LD_FLAGS) \
		./cmd/$(APPLIER_NAME)-azure \
		--kubeconfig=$(KUBECONFIG) \
		--infrastructure-config=dev/credentials.yaml

.PHONY: start-failedvm-simulator-azure
VM_NAME = ""
start-failedvm-simulator-azure:
	@GO111MODULE=on go run \
		-ldflags $(LD_FLAGS) \
		./cmd/failedvm-simulator-azure \
		$(VM_NAME)

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@LD_FLAGS="-w -X github.com/gardener/$(NAME)/pkg/version.Version=$(VERSION) -X github.com/gardener/$(NAME)/pkg/version.GitCommit=$(shell git rev-parse --verify HEAD) -X github.com/gardener/$(NAME)/pkg/version.BuildDate=$(shell date --rfc-3339=seconds | sed 's/ /T/')" \
	bash $(GARDENER_HACK_DIR)/install.sh ./...

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
	@python3 -m venv $(REPO_ROOT)/.env
	@. $(REPO_ROOT)/.env/bin/activate && pip3 install --upgrade pip && pip3 install -r $(REPO_ROOT)/test/requirements.txt

.PHONY: tidy
tidy:
	@GO111MODULE=on go mod tidy
	@mkdir -p $(REPO_ROOT)/.ci/hack && cp $(GARDENER_HACK_DIR)/.ci/* $(REPO_ROOT)/.ci/hack/ && chmod +xw $(REPO_ROOT)/.ci/hack/*
	@GARDENER_HACK_DIR=$(GARDENER_HACK_DIR) $(REPO_ROOT)/hack/update-github-templates.sh

.PHONY: clean
clean:
	@bash $(GARDENER_HACK_DIR)/clean.sh ./cmd/... ./pkg/... ./test/...

.PHONY: check-generate
check-generate:
	@bash $(GARDENER_HACK_DIR)/check-generate.sh $(REPO_ROOT)

.PHONY: check
check: $(GOIMPORTS) $(GOLANGCI_LINT) $(HELM)
	@REPO_ROOT=$(REPO_ROOT) bash $(GARDENER_HACK_DIR)/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./pkg/...
	@REPO_ROOT=$(REPO_ROOT) bash $(GARDENER_HACK_DIR)/check-charts.sh ./charts
	@. $(REPO_ROOT)/.env/bin/activate && flake8 $(REPO_ROOT)/test

.PHONY: generate
generate: $(VGOPATH) $(GEN_CRD_API_REFERENCE_DOCS) $(MOCKGEN)
	@REPO_ROOT=$(REPO_ROOT) VGOPATH=$(VGOPATH) GARDENER_HACK_DIR=$(GARDENER_HACK_DIR) bash $(GARDENER_HACK_DIR)/generate-sequential.sh ./cmd/... ./pkg/...
	$(MAKE) format

.PHONY: format
format: $(GOIMPORTS) $(GOIMPORTSREVISER)
	@bash $(GARDENER_HACK_DIR)/format.sh ./cmd ./pkg

.PHONY: test
test:
	@bash $(GARDENER_HACK_DIR)/test.sh ./cmd/... ./pkg/...

.PHONY: test-cov
test-cov:
	@bash $(GARDENER_HACK_DIR)/test-cover.sh ./cmd/... ./pkg/...

.PHONY: test-clean
test-clean:
	@bash $(GARDENER_HACK_DIR)/test-cover-clean.sh

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

