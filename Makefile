.PHONY: test
test:
	go test ./...

.PHONY: test-clean
test-clean:
	go clean -testcache
	go test ./...

bin:
	mkdir .bin

VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(VERSION)'

.PHONY: build
build: bin
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu ./cmd/

.PHONY: server
server: build
	go run -ldflags="$(LDFLAGS)" ./cmd/ server
