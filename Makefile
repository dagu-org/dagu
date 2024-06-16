.PHONY: build server scheduler test proto certs swagger https

########## Variables ##########
SRC_DIR=./
DST_DIR=$(SRC_DIR)/internal
BUILD_VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(BUILD_VERSION)'

VERSION=
DOCKER_CMD := docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7,linux/arm64/v8 --builder container --build-arg VERSION=$(VERSION) --push --no-cache

DEV_CERT_SUBJ_CA="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=DAGU/CN=*.dagu.dev/emailAddress=ca@dev.com"
DEV_CERT_SUBJ_SERVER="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=SERVER/CN=*.server.dev/emailAddress=server@dev.com"
DEV_CERT_SUBJ_CLIENT="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=CLIENT/CN=*.client.dev/emailAddress=client@dev.com"
DEV_CERT_SUBJ_ALT="subjectAltName=DNS:localhost"

PKG_SWAGGER=github.com/go-swagger/go-swagger/cmd/swagger
PKG_GOLANGCI_LINT=github.com/golangci/golangci-lint/cmd/golangci-lint

COLOR_GREEN=\033[0;32m
COLOR_RESET=\033[0m

FRONTEND_DIR=./internal/service/frontend
FRONTEND_GEN_DIR=${FRONTEND_DIR}/gen
FRONTEND_ASSETS_DIR=${FRONTEND_DIR}/assets
CERT_DIR=./cert

FRONTEND_BUILD_DIR=./ui/dist

APP_NAME=dagu
BIN_DIR=./bin
BIN_NAME=${BIN_DIR}/${APP_NAME}

########## Main Targets ##########

# main starts the backend server.
main: build-ui
	go run . server

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
	@go test --race ./...

# test-clean cleans the test cache and run all tests.
test-clean:
	@go clean -testcache
	@go test --race ./...

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
	${BIN_NAME} server

# scheduler build the binary and start the scheduler.
scheduler: golangci-lint build-bin
	${BIN_NAME} scheduler

########## Tools ##########

build-bin: golangci-lint
	@mkdir -p ${BIN_DIR}
	@go build -ldflags="$(LDFLAGS)" -o ${BIN_NAME} .

build-ui:
	@echo "${COLOR_GREEN}Building UI...${COLOR_RESET}"
	@cd ui; \
		yarn && yarn build

	@rm -f ${FRONTEND_ASSETS_DIR}/*.js
	@rm -f ${FRONTEND_ASSETS_DIR}/*.woff
	@rm -f ${FRONTEND_ASSETS_DIR}/*.woff2

	@cp ${FRONTEND_BUILD_DIR}/*.js ${FRONTEND_ASSETS_DIR}
	@cp ${FRONTEND_BUILD_DIR}/*.woff ${FRONTEND_ASSETS_DIR}
	@cp ${FRONTEND_BUILD_DIR}/*.woff2 ${FRONTEND_ASSETS_DIR}

golangci-lint:
	@echo "${COLOR_GREEN}Installing golangci-lint...${COLOR_RESET}"
	@go install $(PKG_GOLANGCI_LINT)
	@echo "${COLOR_GREEN}Running golangci-lint...${COLOR_RESET}"
	@golangci-lint run ./...

clean-swagger:
	@echo "${COLOR_GREEN}Cleaning swagger...${COLOR_RESET}"
	@rm -rf ${FRONTEND_GEN_DIR}/restapi/models
	@rm -rf ${FRONTEND_GEN_DIR}/restapi/operations

gen-swagger:
	@echo "${COLOR_GREEN}Installing swagger...${COLOR_RESET}"
	@go install $(PKG_SWAGGER)
	@echo "${COLOR_GREEN}Validating swagger...${COLOR_RESET}"
	@swagger validate ./swagger.yaml
	@echo "${COLOR_GREEN}Generating swagger...${COLOR_RESET}"
	@swagger generate server -t ${FRONTEND_GEN_DIR} --server-package=restapi --exclude-main -f ./swagger.yaml
	@echo "${COLOR_GREEN}Running go mod tidy...${COLOR_RESET}"
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

