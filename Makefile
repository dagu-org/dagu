VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(VERSION)'

.PHONY: build-dir
build-dir:
	mkdir -p ./bin

.PHONY: build
build: build-admin build-dir build-bin

.PHONY: build-admin
build-admin:
	cd admin; \
		yarn && yarn build
	cp admin/dist/bundle.js ./internal/admin/handlers/web/assets/js/

.PHONY: build-bin
build-bin:
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .

.PHONY: server
server: build-dir
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .
	./bin/dagu server

.PHONY: scheduler
scheduler: build-dir
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .
	./bin/dagu scheduler

.PHONY: test
test:
	go test -v ./...

.PHONY: test-clean
test-clean:
	go clean -testcache
	go test -v ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: clean
clean:
	rm -rf bin admin/dist