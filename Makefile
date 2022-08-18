VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(VERSION)'

.PHONY: build-dir
build-dir:
	mkdir -p ./bin

.PHONY: build
build: build-admin build-dir
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu ./cmd/

.PHONY: build-admin
build-admin:
	cd admin; \
		yarn && yarn build
	cp admin/dist/bundle.js ./internal/admin/handlers/web/assets/js/

.PHONY: server
server: build-dir
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu ./cmd/
	./bin/dagu server

.PHONY: scheduler
scheduler: build-dir
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu ./cmd/
	./bin/dagu scheduler

.PHONY: test
test: build
	go test ./...

.PHONY: test-clean
test-clean:
	go clean -testcache
	go test ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: clean
clean:
	rm -rf bin admin/dist