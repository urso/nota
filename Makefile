.PHONY: all build fmt check test clean nvim-lint nvim-check nvim-test

all: fmt build test

build:
	go build -o nota ./cmd/nota

fmt:
	gofmt -w .

check:
	gofmt -l .
	go vet ./...

test:
	go test ./...

clean:
	rm -f nota

nvim-lint:
	cd nvim && selene lua/

nvim-check:
	cd nvim && lua-language-server --configpath "$$(pwd)/.luarc.json" --check lua/

nvim-test:
	./nvim/run-tests.sh
