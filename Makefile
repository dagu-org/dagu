.PHONY: build server scheduler test proto certs swagger

########## Variables ##########
SRC_DIR=./
DST_DIR=$(SRC_DIR)/internal
BUILD_VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(BUILD_VERSION)'

VERSION=
DOCKER_CMD := docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --build-arg VERSION=$(VERSION) --push --no-cache

DEV_CERT_SUBJ_CA="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=DAGU/CN=*.dagu.dev/emailAddress=ca@dev.com"
DEV_CERT_SUBJ_SERVER="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=SERVER/CN=*.server.dev/emailAddress=server@dev.com"
DEV_CERT_SUBJ_CLIENT="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=CLIENT/CN=*.client.dev/emailAddress=client@dev.com"
DEV_CERT_SUBJ_ALT="subjectAltName=DNS:localhost"

########## Main Targets ##########
main:
	go run . server

watch:
	nodemon --watch . --ext go,gohtml --verbose --signal SIGINT --exec 'make server'

test:
	@go test ./...

test-clean:
	@go clean -testcache
	@go test ./...

install-tools: install-protobuf install-nodemon install-swagger

proto: gen-pb

swagger: clean-swagger gen-swagger

certs: cert-dir gencerts-ca gencerts-server gencerts-client gencert-check

build: build-ui build-dir gen-pb go-lint build-bin

build-image:
ifeq ($(VERSION),)
	$(error "VERSION is null")
endif
	$(DOCKER_CMD) -t yohamta/dagu:$(VERSION) .
	$(DOCKER_CMD) -t yohamta/dagu:latest .

server: go-lint build-dir build-bin
	./bin/dagu server

https-server:
	@DAGU_CERT_FILE=./cert/server-cert.pem \
		DAGU_KEY_FILE=./cert/server-key.pem \
		go run . server

scheduler: go-lint build-dir build-bin
	./bin/dagu scheduler

########## Tools ##########

gen-pb:
	protoc -I=$(SRC_DIR) --go_out=$(DST_DIR) $(SRC_DIR)/internal/proto/*.proto

build-bin:
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .

build-dir:
	@mkdir -p ./bin

build-ui:
	@cd ui; \
		yarn && yarn build

	@rm -f ./service/frontend/assets/*.js
	@rm -f ./service/frontend/assets/*.woff
	@rm -f ./service/frontend/assets/*.woff2

	@cp ui/dist/*.js ./service/frontend/assets/
	@cp ui/dist/*.woff ./service/frontend/assets/
	@cp ui/dist/*.woff2 ./service/frontend/assets/

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
	@rm -rf service/frontend/restapi/models
	@rm -rf service/frontend/restapi/operations

gen-swagger:
	@echo "Validating swagger yaml"
	@swagger validate ./swagger.yaml
	@echo "Generating swagger server code from yaml"
	@swagger generate server -t service/frontend --server-package=restapi --exclude-main -f ./swagger.yaml
	@echo "Running go mod tidy"
	@go mod tidy

install-protobuf:
	brew install protobuf
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

install-nodemon:
	npm install -g nodemon

install-swagger:
	brew tap go-swagger/go-swagger
	brew install go-swagger
