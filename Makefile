# Copyright the PeerFintech. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# Supported Targets:

# unit-test: runs all the unit tests

# Build flags (overridable)
GO_LDFLAGS                 ?=

## integration-test
GO_CMD                 ?= go
MAKEFILE_THIS          := $(lastword $(MAKEFILE_LIST))
THIS_PATH              := $(patsubst %/,%,$(dir $(abspath $(MAKEFILE_THIS))))
TEST_SCRIPTS_PATH      := test/scripts
TEST_FIXTURES_PATH     := test/fixtures
TEST_E2E_PATH          := github.com/zhj0811/gohfc/test/integration/e2e
IMAGE_TAG              := 2.0.0
GOHFC_PATH := $(THIS_PATH)
export GOHFC_PATH
export IMAGE_TAG
export GO_CMD

.PHONY: integration-test
integration-test: dockerupfabric downloadvendor lifecycle-test

.PHONY: dockerupfabric
dockerupfabric:
	@echo "integration-test rely on fabric2.X, default image_tag:2.0.0"
	docker-compose -f $(TEST_FIXTURES_PATH)/docker-compose-solo.yaml up -d


.PHONY: downloadvendor
downloadvendor:
	@go mod vendor && cd $(TEST_FIXTURES_PATH)/chaincode && go mod vendor

.PHONY: setup-test
setup-test:
	go test -v  -run=TestE2E $(TEST_E2E_PATH) -mod=vendor

.PHONY: clienthandler-test
clienthandler-test:
	go test -v  -run=TestFabricClientHandler $(TEST_E2E_PATH) -mod=vendor

.PHONY: lifecycle-test
lifecycle-test:
	go test -v  -run=TestLifecycle $(TEST_E2E_PATH) -mod=vendor

.PHONY: withoutset-test
withoutset-test:
	@echo "please make sure that integration-test operation has been performed"
	go test -v -run=TestRunWithoutSet $(TEST_E2E_PATH)

.PHONY: discover-test
discover-test:
	@echo "please make sure that integration-test operation has been performed"
	go test -v -run=TestDiscover $(TEST_E2E_PATH)


.PHONY: clean
clean: clean-integration-test

.PHONY:clean-integration-test
clean-integration-test:
	docker-compose -f $(TEST_FIXTURES_PATH)/docker-compose-solo.yaml down
	-@docker ps -a|grep "dev\-peer"|awk '{print $$1}'|xargs docker rm -f
	-@docker images |grep "^dev\-peer"|awk '{print $$3}'|xargs docker rmi -f

.PHONY: unit-test
unit-test:
	@MODULE="github.com/zhj0811/gohfc" \
	PKG_ROOT="./pkg" \
	$(TEST_SCRIPTS_PATH)/unit.sh
