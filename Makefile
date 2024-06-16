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

########## Main Targets ##########

# main starts the backend server.
main:
	go run . server

# https starts the server with the HTTPS protocol.
https:
	@DAGU_CERT_FILE=./cert/server-cert.pem \
		DAGU_KEY_FILE=./cert/server-key.pem \
		go run . server

# watch starts development UI server.
# The backend server should be running.
watch:
	nodemon --watch . --ext go,gohtml --verbose --signal SIGINT --exec 'make server'

# test runs all tests.
test:
	@go test --race ./...

# test-clean cleans the test cache and run all tests.
test-clean:
	@go clean -testcache
	@go test --race ./...

# lint the Go code.
lint:
	golangci-lint run -v

# install-tools installs the required tools.
install-tools: install-nodemon install-swagger

# swagger generates the swagger server code.
swagger: clean-swagger gen-swagger

# certs generates the certificates to use in the development environment.
certs: cert-dir gencerts-ca gencerts-server gencerts-client gencert-check

# build build the binary.
build: build-ui build-dir go-lint build-bin

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
	$(DOCKER_CMD) -t ghcr.io/dagu-dev/dagu:$(VERSION) .

# build-image-latest build the docker image with the latest tag and push to 
# the registry.
build-image-latest:
	$(DOCKER_CMD) -t ghcr.io/dagu-dev/dagu:latest .

# server build the binary and start the server.
server: go-lint build-dir build-bin
	./bin/dagu server

# scheduler build the binary and start the scheduler.
scheduler: go-lint build-dir build-bin
	./bin/dagu scheduler

########## Tools ##########

build-bin:
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .

build-dir:
	@mkdir -p ./bin

build-ui:
	@cd ui; \
		yarn && yarn build

	@rm -f ./internal/service/frontend/assets/*.js
	@rm -f ./internal/service/frontend/assets/*.woff
	@rm -f ./internal/service/frontend/assets/*.woff2

	@cp ui/dist/*.js ./internal/service/frontend/assets/
	@cp ui/dist/*.woff ./internal/service/frontend/assets/
	@cp ui/dist/*.woff2 ./internal/service/frontend/assets/

go-lint:
	@golangci-lint run ./...

cert-dir:
	@mkdir -p ./cert

gencerts-ca:
	@openssl req -x509 -newkey rsa:4096 \
		-nodes -days 365 -keyout cert/ca-key.pem \
		-out cert/ca-cert.pem \
		-subj "$(DEV_CERT_SUBJ_CA)"

gencerts-server:
	@openssl req -newkey rsa:4096 -nodes -keyout cert/server-key.pem \
		-out cert/server-req.pem \
		-subj "$(DEV_CERT_SUBJ_SERVER)"

	@openssl x509 -req -in cert/server-req.pem -CA cert/ca-cert.pem -CAkey cert/ca-key.pem \
		-CAcreateserial -out cert/server-cert.pem \
		-extfile cert/openssl.conf

gencerts-client:
	@openssl req -newkey rsa:4096 -nodes -keyout cert/client-key.pem \
		-out cert/client-req.pem \
		-subj "$(DEV_CERT_SUBJ_CLIENT)"

	@openssl x509 -req -in cert/client-req.pem -days 60 -CA cert/ca-cert.pem \
		-CAkey cert/ca-key.pem -CAcreateserial -out cert/client-cert.pem \
		-extfile cert/openssl.conf

gencert-check:
	@openssl x509 -in cert/server-cert.pem -noout -text

clean-swagger:
	@echo "Cleaning files"
	@rm -rf service/frontend/gen/restapi/models
	@rm -rf service/frontend/gen/restapi/operations

gen-swagger:
	@echo "Validating swagger yaml"
	@swagger validate ./swagger.yaml
	@echo "Generating swagger server code from yaml"
	@swagger generate server -t service/frontend/gen --server-package=restapi --exclude-main -f ./swagger.yaml
	@echo "Running go mod tidy"
	@go mod tidy

install-nodemon:
	npm install -g nodemon

install-swagger:
	brew tap go-swagger/go-swagger
	brew install go-swagger
