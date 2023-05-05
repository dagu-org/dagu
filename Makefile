VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(VERSION)'

.PHONY: build server scheduler test

main:
	@go run main.go server

build-dir:
	@mkdir -p ./bin

build: build-admin build-dir build-bin

build-admin:
	@cd admin; \
		yarn && yarn build
	@cp admin/dist/bundle.js ./internal/admin/handlers/web/assets/js/

build-bin:
	@go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .

server:
	@go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .
	@./bin/dagu server

scheduler: build-dir
	@go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .
	@./bin/dagu scheduler

test:
	@go test -v ./...

test-clean:
	@go clean -testcache
	@go test -v ./...

lint:
	@golangci-lint run ./...

clean:
	@rm -rf bin admin/dist

DAGU_VERSION=
BUILD_IMAGE_ARGS := buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --build-arg VERSION=$(DAGU_VERSION) --push --no-cache
build-image:
ifeq ($(DAGU_VERSION),)
	$(error "DAGU_VERSION is null")
endif
	docker $(BUILD_IMAGE_ARGS) -t yohamta/dagu:$(DAGU_VERSION) .
	docker $(BUILD_IMAGE_ARGS) -t yohamta/dagu:latest .

DEV_CERT_SUBJ_CA="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=DAGU/CN=*.dagu.dev/emailAddress=ca@dev.com"
DEV_CERT_SUBJ_SERVER="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=SERVER/CN=*.server.dev/emailAddress=server@dev.com"
DEV_CERT_SUBJ_CLIENT="/C=TR/ST=ASIA/L=TOKYO/O=DEV/OU=CLIENT/CN=*.client.dev/emailAddress=client@dev.com"
DEV_CERT_SUBJ_ALT="subjectAltName=DNS:localhost"

# Commands to generate self-signed certificates for development
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
