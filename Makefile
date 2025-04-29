##############################################################################
# Arguments
##############################################################################

VERSION=

##############################################################################
# Variables
##############################################################################

# This Makefile's directory
SCRIPT_DIR=$(abspath $(dir $(lastword $(MAKEFILE_LIST))))

# Directories for miscellaneous files for the local environment
LOCAL_DIR=$(SCRIPT_DIR)/.local
LOCAL_BIN_DIR=$(LOCAL_DIR)/bin

# Configuration directory
CONFIG_DIR=$(SCRIPT_DIR)/config

# Local build settings
BIN_DIR=$(SCRIPT_DIR)/.local/bin
BUILD_VERSION=$(shell git describe --tags)
DATE=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(BUILD_VERSION)-$(DATE)'

# Application name
APP_NAME=dagu

# Docker image build configuration
DOCKER_CMD := docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7,linux/arm64/v8 --builder container --build-arg LDFLAGS="$(LDFLAGS)" --push --no-cache

# Arguments for the tests
GOTESTSUM_ARGS=--format=standard-quiet
GO_TEST_FLAGS=-v --race

# OpenAPI configuration
OAPI_SPEC_DIR_V2=./api/v2
OAPI_SPEC_FILE_V2=${OAPI_SPEC_DIR_V2}/api.yaml
OAPI_CONFIG_FILE_V2=${OAPI_SPEC_DIR_V2}/config.yaml

OAPI_SPEC_DIR_V1=./api/v1
OAPI_SPEC_FILE_V1=${OAPI_SPEC_DIR_V1}/api.yaml
OAPI_CONFIG_FILE_V1=${OAPI_SPEC_DIR_V1}/config.yaml

# Frontend directories

FE_DIR=./internal/frontend
FE_GEN_DIR=${FE_DIR}/gen
FE_ASSETS_DIR=${FE_DIR}/assets
FE_BUILD_DIR=./ui/dist
FE_BUNDLE_JS=${FE_ASSETS_DIR}/bundle.js

# Colors for the output

COLOR_GREEN=\033[0;32m
COLOR_RESET=\033[0m
COLOR_RED=\033[0;31m

# Go packages for the tools

PKG_golangci_lint=github.com/golangci/golangci-lint/v2/cmd/golangci-lint
PKG_gotestsum=gotest.tools/gotestsum
PKG_addlicense=github.com/google/addlicense
PKG_changelog-from-release=github.com/rhysd/changelog-from-release/v3@latest
PKG_oapi_codegen=github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
PKG_kin_openapi_validate=github.com/getkin/kin-openapi/cmd/validate

# Certificates for the development environment

CERTS_DIR=${LOCAL_DIR}/certs

DEV_CERT_SUBJ_CA="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=DAGU/CN=*.dagu.dev/emailAddress=ca@dev.com"
DEV_CERT_SUBJ_SERVER="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=SERVER/CN=*.server.dev/emailAddress=server@dev.com"
DEV_CERT_SUBJ_CLIENT="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=CLIENT/CN=*.client.dev/emailAddress=client@dev.com"
DEV_CERT_SUBJ_ALT="subjectAltName=DNS:localhost"

CA_CERT_FILE=${CERTS_DIR}/ca-cert.pem
CA_KEY_FILE=${CERTS_DIR}/ca-key.pem
SERVER_CERT_REQ=${CERTS_DIR}/server-req.pem
SERVER_CERT_FILE=${CERTS_DIR}/server-cert.pem
SERVER_KEY_FILE=${CERTS_DIR}/server-key.pem
CLIENT_CERT_REQ=${CERTS_DIR}/client-req.pem
CLIENT_CERT_FILE=${CERTS_DIR}/client-cert.pem
CLIENT_KEY_FILE=${CERTS_DIR}/client-key.pem
OPENSSL_CONF=${CONFIG_DIR}/openssl.local.conf

##############################################################################
# Targets
##############################################################################

# run starts the frontend server and the scheduler.
.PHONY: run
run: ${FE_BUNDLE_JS}
	@echo "${COLOR_GREEN}Starting the frontend server and the scheduler...${COLOR_RESET}"
	@go run ./cmd start-all

# server build the binary and start the server.
.PHONY: run-server
run-server: golangci-lint bin
	@echo "${COLOR_GREEN}Starting the server...${COLOR_RESET}"
	${LOCAL_BIN_DIR}/${APP_NAME} server

# scheduler build the binary and start the scheduler.
.PHONY: run-scheduler
run-scheduler: golangci-lint bin
	@echo "${COLOR_GREEN}Starting the scheduler...${COLOR_RESET}"
	${LOCAL_BIN_DIR}/${APP_NAME} scheduler

# check if the frontend assets are built.
${FE_BUNDLE_JS}:
	@echo "${COLOR_RED}Error: frontend assets are not built.${COLOR_RESET}"
	@echo "${COLOR_RED}Please run 'make ui' before starting the server.${COLOR_RESET}"

# https starts the server with the HTTPS protocol.
.PHONY: run-server-https
run-server-https: ${SERVER_CERT_FILE} ${SERVER_KEY_FILE}
	@echo "${COLOR_GREEN}Starting the server with HTTPS...${COLOR_RESET}"
	@DAGU_CERT_FILE=${SERVER_CERT_FILE} \
		DAGU_KEY_FILE=${SERVER_KEY_FILE} \
		go run ./cmd start-all

# test runs all tests.
.PHONY: test
test: bin
	@echo "${COLOR_GREEN}Running tests...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install ${PKG_gotestsum}
	@go clean -testcache
	@${LOCAL_BIN_DIR}/gotestsum ${GOTESTSUM_ARGS} -- ${GO_TEST_FLAGS} ./...

# test-coverage runs all tests with coverage.
.PHONY: test-coverage
test-coverage:
	@echo "${COLOR_GREEN}Running tests with coverage...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install ${PKG_gotestsum}
	@${LOCAL_BIN_DIR}/gotestsum ${GOTESTSUM_ARGS} -- ${GO_TEST_FLAGS} -coverprofile="coverage.txt" -covermode=atomic ./...

# open-coverage opens the coverage file
.PHONY: open-coverage
open-coverage:
	@go tool cover -html=coverage.txt

# lint runs the linter.
.PHONY: lint
lint: golangci-lint

# api generates the server code from the OpenAPI specification.
.PHONY: api
api: api-validate
	@echo "${COLOR_GREEN}Generating API...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install ${PKG_oapi_codegen}
	@${LOCAL_BIN_DIR}/oapi-codegen --config=${OAPI_CONFIG_FILE_V2} ${OAPI_SPEC_FILE_V2}

# api-validate validates the OpenAPI specification.
.PHONY: api-validate
api-validate:
	@echo "${COLOR_GREEN}Validating API...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install ${PKG_kin_openapi_validate}
	@${LOCAL_BIN_DIR}/validate ${OAPI_SPEC_FILE_V2}

# api-v1 generates the server code from the OpenAPI specification.
.PHONY: apiv1
apiv1: apiv1-validate
	@echo "${COLOR_GREEN}Generating API...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install ${PKG_oapi_codegen}
	@${LOCAL_BIN_DIR}/oapi-codegen --config=${OAPI_CONFIG_FILE_V1} ${OAPI_SPEC_FILE_V1}

# api-validate-v1 validates the OpenAPI specification.
.PHONY: apiv1-validate
apiv1-validate:
	@echo "${COLOR_GREEN}Validating API...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install ${PKG_kin_openapi_validate}
	@${LOCAL_BIN_DIR}/validate ${OAPI_SPEC_FILE_V1}

# api generates the swagger server code.
.PHONY: swagger
swagger: clean-swagger gen-swagger

# certs generates the certificates to use in the development environment.
.PHONY: certs
certs: ${CERTS_DIR} ${SERVER_CERT_FILE} ${CLIENT_CERT_FILE} certs-check

# build build the binary.
.PHONY: build
build: ui bin

# build-image build the docker image and push to the registry.
# VERSION should be set via the argument as follows:
# ```sh
# make build-image VERSION={version}
# ```
# {version} should be the version number such as "1.13.0".

.PHONY: build-image
build-image: build-image-version

.PHONY: build-image-latest
build-image-version:
ifeq ($(VERSION),)
	$(error "VERSION is not set")
endif
	echo "${COLOR_GREEN}Building the docker image with the version $(VERSION)...${COLOR_RESET}"
	$(DOCKER_CMD) -t ghcr.io/dagu-org/${APP_NAME}:$(VERSION) -t ghcr.io/dagu-org/${APP_NAME}:latest .

# build-image-latest build the docker image with the latest tag and push to 
# the registry.
.PHONY: build-image-latest
build-image-latest:
	@echo "${COLOR_GREEN}Building the docker image...${COLOR_RESET}"
	$(DOCKER_CMD) -t ghcr.io/dagu-org/${APP_NAME}:latest .

${LOCAL_DIR}/merged:
	@mkdir -p ${LOCAL_DIR}/merged

# addlicense adds license header to all files.
.PHONY: addlicense
addlicense:
	@echo "${COLOR_GREEN}Adding license headers...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install ${PKG_addlicense}
	@${LOCAL_BIN_DIR}/addlicense \
		-ignore "**/node_modules/**" \
		-ignore "./**/gen/**" \
		-ignore "Dockerfile" \
		-ignore "ui/*" \
		-ignore "ui/**/*" \
		-ignore "bin/*" \
		-ignore "local/*" \
		-ignore "docs/**" \
		-ignore "**/examples/*" \
		-ignore ".github/*" \
		-ignore ".github/**/*" \
		-ignore ".*" \
		-ignore "**/*.yml" \
		-ignore "**/*.yaml" \
		-ignore "**/filenotify/*" \
		-ignore "**/testdata/**" \
		-c "Yota Hamada" \
		-f scripts/header.txt \
		.

##############################################################################
# Internal targets
##############################################################################

# bin builds the go application.
.PHONY: bin
bin:
	@echo "${COLOR_GREEN}Building the binary...${COLOR_RESET}"
	@mkdir -p ${BIN_DIR}
	@go build -ldflags="$(LDFLAGS)" -o ${BIN_DIR}/${APP_NAME} ./cmd

.PHONY: ui
# ui builds the frontend codes.
ui: clean-ui build-ui cp-assets

# build-ui builds the frontend codes.
.PHONY: build-ui
build-ui:
	@echo "${COLOR_GREEN}Building UI...${COLOR_RESET}"
	@cd ui; \
		pnpm install; \
		NODE_OPTIONS="--max-old-space-size=8192" pnpm webpack --config webpack.dev.js; \
		pnpm webpack --config webpack.prod.js
	@echo "${COLOR_GREEN}Waiting for the build to finish...${COLOR_RESET}"
	@sleep 3 # wait for the build to finish

# build-ui-prod builds the frontend codes for production.
.PHONY: cp-assets
cp-assets:
	@echo "${COLOR_GREEN}Copying UI assets...${COLOR_RESET}"
	@rm -f ${FE_ASSETS_DIR}/*
	@cp ${FE_BUILD_DIR}/* ${FE_ASSETS_DIR}

# clean-ui removes the UI build cache.
.PHONY: clean-ui
clean-ui:
	@echo "${COLOR_GREEN}Cleaning UI build cache...${COLOR_RESET}"
	@cd ui; \
		rm -rf node_modules; \
		rm -rf .cache;

# golangci-lint run linting tool.
.PHONY: golangci-lint
golangci-lint:
	@echo "${COLOR_GREEN}Running linter...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install $(PKG_golangci_lint)
	@${LOCAL_BIN_DIR}/golangci-lint run --fix ./...

# changelog generates a changelog from the releases.
.PHONY: changelog
changelog:
	@echo "${COLOR_GREEN}Running changelog...${COLOR_RESET}"
	@GOBIN=${LOCAL_BIN_DIR} go install $(PKG_changelog-from-release)
	@${LOCAL_BIN_DIR}/changelog-from-release -c > CHANGELOG.md

##############################################################################
# Certificates
##############################################################################

${CA_CERT_FILE}:
	@echo "${COLOR_GREEN}Generating CA certificates...${COLOR_RESET}"
	@openssl req -x509 -newkey rsa:4096 \
		-nodes -days 365 -keyout ${CA_KEY_FILE} \
		-out ${CA_CERT_FILE} \
		-subj "$(DEV_CERT_SUBJ_CA)"

${SERVER_KEY_FILE}:
	@echo "${COLOR_GREEN}Generating server key...${COLOR_RESET}"
	@openssl req -newkey rsa:4096 -nodes -keyout ${SERVER_KEY_FILE} \
		-out ${SERVER_CERT_REQ} \
		-subj "$(DEV_CERT_SUBJ_SERVER)"

${SERVER_CERT_FILE}: ${CA_CERT_FILE} ${SERVER_KEY_FILE}
	@echo "${COLOR_GREEN}Generating server certificate...${COLOR_RESET}"
	@openssl x509 -req -in ${SERVER_CERT_REQ} -CA ${CA_CERT_FILE} -CAkey ${CA_KEY_FILE} \
		-CAcreateserial -out ${SERVER_CERT_FILE} \
		-extfile ${OPENSSL_CONF}

${CLIENT_KEY_FILE}:
	@echo "${COLOR_GREEN}Generating client key...${COLOR_RESET}"
	@openssl req -newkey rsa:4096 -nodes -keyout ${CLIENT_KEY_FILE} \
		-out ${CLIENT_CERT_REQ} \
		-subj "$(DEV_CERT_SUBJ_CLIENT)"

${CLIENT_CERT_FILE}: ${CA_CERT_FILE} ${CLIENT_KEY_FILE}
	@echo "${COLOR_GREEN}Generating client certificate...${COLOR_RESET}"
	@openssl x509 -req -in ${CLIENT_CERT_REQ} -days 60 -CA ${CA_CERT_FILE} \
		-CAkey ${CA_KEY_FILE} -CAcreateserial -out ${CLIENT_CERT_FILE} \
		-extfile ${OPENSSL_CONF}

${CERTS_DIR}:
	@echo "${COLOR_GREEN}Creating the certificates directory...${COLOR_RESET}"
	@mkdir -p ${CERTS_DIR}

.PHONY: certs-check
certs-check:
	@echo "${COLOR_GREEN}Checking CA certificate...${COLOR_RESET}"
	@openssl x509 -in ${SERVER_CERT_FILE} -noout -text
