.PHONY: test
test:
	go test ./...

.PHONY: test-clean
test-clean:
	go clean -testcache
	go test ./...

bin:
	mkdir .bin

.PHONY: build
build: bin
	go build -o ./bin/dagman ./cmd/

.PHONY: server
server: build
	go run ./cmd/ server
