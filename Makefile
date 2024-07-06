.PHONY: build server scheduler test proto certs swagger https

########## Arguments ##########

VERSION=

########## Variables ##########

# This Makefile's directory
SCRIPT_DIR=$(abspath $(dir $(lastword $(MAKEFILE_LIST))))

SRC_DIR=$(SCRIPT_DIR)
DST_DIR=$(SRC_DIR)/internal

BUILD_VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(BUILD_VERSION)'

DOCKER_CMD := docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7,linux/arm64/v8 --builder container --build-arg VERSION=$(VERSION) --push --no-cache

DEV_CERT_SUBJ_CA="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=DAGU/CN=*.dagu.dev/emailAddress=ca@dev.com"
DEV_CERT_SUBJ_SERVER="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=SERVER/CN=*.server.dev/emailAddress=server@dev.com"
DEV_CERT_SUBJ_CLIENT="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=CLIENT/CN=*.client.dev/emailAddress=client@dev.com"
DEV_CERT_SUBJ_ALT="subjectAltName=DNS:localhost"

PKG_SWAGGER=github.com/go-swagger/go-swagger/cmd/swagger
PKG_GOLANGCI_LINT=github.com/golangci/golangci-lint/cmd/golangci-lint
PKG_gotestsum=gotest.tools/gotestsum

COLOR_GREEN=\033[0;32m
COLOR_RESET=\033[0m

FE_DIR=./internal/frontend
FE_GEN_DIR=${FE_DIR}/gen
FE_ASSETS_DIR=${FE_DIR}/assets

CERT_DIR=${SCRIPT_DIR}/cert

FE_BUILD_DIR=./ui/dist
FE_BUNDLE_JS=${FE_ASSETS_DIR}/bundle.js

APP_NAME=dagu
BIN_DIR=${SCRIPT_DIR}/bin

# gotestsum args
GOTESTSUM_ARGS=--format=standard-quiet
GO_TEST_FLAGS=-v --race

########## Main Targets ##########

# run starts the frontend server and the scheduler.
run: ${FE_BUNDLE_JS}
	go run . start-all

# check if the frontend assets are built.
${FE_BUNDLE_JS}:
	echo "Please run 'make build-ui' to build the frontend assets."

# https starts the server with the HTTPS protocol.
https:
	@DAGU_CERT_FILE=${CERT_DIR}/server-cert.pem \
		DAGU_KEY_FILE=${CERT_DIR}/server-key.pem \
		go run . server

# watch starts development UI server.
# The backend server should be running.
watch:
	@echo "${COLOR_GREEN}Installing nodemon...${COLOR_RESET}"
	@npm install -g nodemon
	@nodemon --watch . --ext go,gohtml --verbose --signal SIGINT --exec 'make server'

# test runs all tests.
test:
	@go install ${PKG_gotestsum}
	@gotestsum ${GOTESTSUM_ARGS} -- ${GO_TEST_FLAGS} ./...

# test-coverage runs all tests with coverage.
test-coverage:
	@go install ${PKG_gotestsum}
	@gotestsum ${GOTESTSUM_ARGS} -- ${GO_TEST_FLAGS} -coverprofile="coverage.txt" -covermode=atomic ./...

# test-clean cleans the test cache and run all tests.
test-clean: build-bin
	@go install ${PKG_gotestsum}
	@go clean -testcache
	@gotestsum ${GOTESTSUM_ARGS} -- ${GO_TEST_FLAGS} ./...

# lint runs the linter.
lint: golangci-lint

# swagger generates the swagger server code.
swagger: clean-swagger gen-swagger

# certs generates the certificates to use in the development environment.
certs: cert-dir gencerts-ca gencerts-server gencerts-client gencert-check

# build build the binary.
build: build-ui build-bin

# build-image build the docker image and push to the registry.
# VERSION should be set via the argument as follows:
# ```sh
# make build-image VERSION={version}
# ```
# {version} should be the version number such as v1.13.0.
build-image: build-image-version build-image-latest
build-image-version:
ifeq ($(VERSION),)
	$(error "VERSION is null")
endif
	$(DOCKER_CMD) -t ghcr.io/dagu-dev/${APP_NAME}:$(VERSION) .

# build-image-latest build the docker image with the latest tag and push to 
# the registry.
build-image-latest:
	$(DOCKER_CMD) -t ghcr.io/dagu-dev/${APP_NAME}:latest .

# server build the binary and start the server.
server: golangci-lint build-bin
	${BIN_DIR}/${APP_NAME} server

# scheduler build the binary and start the scheduler.
scheduler: golangci-lint build-bin
	${BIN_DIR}/${APP_NAME} scheduler

########## Tools ##########

build-bin:
	@mkdir -p ${BIN_DIR}
	@go build -ldflags="$(LDFLAGS)" -o ${BIN_DIR}/${APP_NAME} .

build-ui:
	@echo "${COLOR_GREEN}Building UI...${COLOR_RESET}"
	@cd ui; \
		yarn && yarn build
	@echo "${COLOR_GREEN}Copying UI assets...${COLOR_RESET}"
	@rm -f ${FE_ASSETS_DIR}/*
	@cp ${FE_BUILD_DIR}/* ${FE_ASSETS_DIR}

golangci-lint:
	@go install $(PKG_GOLANGCI_LINT)
	@golangci-lint run ./...

clean-swagger:
	@rm -rf ${FE_GEN_DIR}/restapi/models
	@rm -rf ${FE_GEN_DIR}/restapi/operations

gen-swagger:
	@go install $(PKG_SWAGGER)
	@swagger validate ./swagger.yaml
	@swagger generate server -t ${FE_GEN_DIR} --server-package=restapi --exclude-main -f ./swagger.yaml
	@go mod tidy

########## Certificates ##########

cert-dir:
	@echo "${COLOR_GREEN}Creating cert directory...${COLOR_RESET}"
	@mkdir -p ${CERT_DIR}

gencerts-ca:
	@echo "${COLOR_GREEN}Generating CA certificates...${COLOR_RESET}"
	@openssl req -x509 -newkey rsa:4096 \
		-nodes -days 365 -keyout ${CERT_DIR}/ca-key.pem \
		-out ${CERT_DIR}/ca-cert.pem \
		-subj "$(DEV_CERT_SUBJ_CA)"

gencerts-server:
	@echo "${COLOR_GREEN}Generating server certificates...${COLOR_RESET}"
	@openssl req -newkey rsa:4096 -nodes -keyout ${CERT_DIR}/server-key.pem \
		-out ${CERT_DIR}/server-req.pem \
		-subj "$(DEV_CERT_SUBJ_SERVER)"

	@echo "${COLOR_GREEN}Adding subjectAltName...${COLOR_RESET}"
	@openssl x509 -req -in ${CERT_DIR}/server-req.pem -CA ${CERT_DIR}/ca-cert.pem -CAkey ${CERT_DIR}/ca-key.pem \
		-CAcreateserial -out ${CERT_DIR}/server-cert.pem \
		-extfile ${CERT_DIR}/openssl.conf

gencerts-client:
	@echo "${COLOR_GREEN}Generating client certificates...${COLOR_RESET}"
	@openssl req -newkey rsa:4096 -nodes -keyout cert/client-key.pem \
		-out cert/client-req.pem \
		-subj "$(DEV_CERT_SUBJ_CLIENT)"

	@echo "${COLOR_GREEN}Adding subjectAltName...${COLOR_RESET}"
	@openssl x509 -req -in cert/client-req.pem -days 60 -CA cert/ca-cert.pem \
		-CAkey cert/ca-key.pem -CAcreateserial -out cert/client-cert.pem \
		-extfile cert/openssl.conf

gencert-check:
	@echo "${COLOR_GREEN}Checking CA certificate...${COLOR_RESET}"
	@openssl x509 -in cert/server-cert.pem -noout -text

