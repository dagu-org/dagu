bin:
	mkdir .bin

VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(VERSION)'

.PHONY: build
build: build-admin bin
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu ./cmd/

.PHONY: build-admin
build-admin:
	cd admin; \
		yarn && yarn build
	cp admin/dist/bundle.js ./internal/admin/handlers/web/assets/js/

.PHONY: server
server:
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu ./cmd/
	go run -ldflags="$(LDFLAGS)" ./cmd/ server

.PHONY: test
test: build
	go test ./...

.PHONY: test-clean
test-clean:
	go clean -testcache
	go test ./...
