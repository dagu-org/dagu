VERSION=$(shell date +'%y%m%d%H%M%S')
LDFLAGS=-X 'main.version=$(VERSION)'

.PHONY: build server scheduler test

build-dir:
	mkdir -p ./bin

build: build-admin build-dir build-bin

build-admin:
	cd admin; \
		yarn && yarn build
	cp admin/dist/bundle.js ./internal/admin/handlers/web/assets/js/

build-bin:
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .

server:
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .
	./bin/dagu server

scheduler: build-dir
	go build -ldflags="$(LDFLAGS)" -o ./bin/dagu .
	./bin/dagu scheduler

test:
	go test -v ./...

test-clean:
	go clean -testcache
	go test -v ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin admin/dist