.PHONY: all build fmt check test clean

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
