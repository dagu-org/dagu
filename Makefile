.PHONY: build server scheduler test

### Variables ###
SRC_DIR=./
DST_DIR=$(SRC_DIR)/internal
BUILD_VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(BUILD_VERSION)'

# parameter for build image
VERSION=
DOCKER_CMD := docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --build-arg VERSION=$(VERSION) --push --no-cache

### Commands ###
watch:
	nodemon --watch . --ext go,gohtml --verbose --signal SIGINT --exec 'make server'

gen-pb:
	protoc -I=$(SRC_DIR) --go_out=$(DST_DIR) $(SRC_DIR)/internal/proto/*.proto

build-bin:
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .

server:
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .
	./bin/dagu server

scheduler: build-dir
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .
	./bin/dagu scheduler

build-dir:
	@mkdir -p ./bin

build: build-ui build-dir gen-pb build-bin

build-ui:
	@cd ui; \
		yarn && yarn build
	@cp ui/dist/*.js ./internal/web/handlers/assets/
	@cp ui/dist/*.woff ./internal/web/handlers/assets/
	@cp ui/dist/*.woff2 ./internal/web/handlers/assets/

test:
	@go test ./...

test-clean:
	@go clean -testcache
	@go test ./...

lint:
	@golangci-lint run ./...

build-image:
ifeq ($(VERSION),)
	$(error "VERSION is null")
endif
	$(DOCKER_CMD) -t yohamta/dagu:$(VERSION) .
	$(DOCKER_CMD) -t yohamta/dagu:latest .

### Commands for self-signed certificates ###

DEV_CERT_SUBJ_CA="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=DAGU/CN=*.dagu.dev/emailAddress=ca@dev.com"
DEV_CERT_SUBJ_SERVER="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=SERVER/CN=*.server.dev/emailAddress=server@dev.com"
DEV_CERT_SUBJ_CLIENT="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=CLIENT/CN=*.client.dev/emailAddress=client@dev.com"
DEV_CERT_SUBJ_ALT="subjectAltName=DNS:localhost"

gen-certs: cert gencerts-ca gencerts-server gencerts-client gencert-check

cert:
	@mkdir ./cert

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

server-tls:
	@DAGU_CERT_FILE=./cert/server-cert.pem DAGU_KEY_FILE=./cert/server-key.pem go run . server

install-tools:
	brew install protobuf
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	npm install -g nodemon