.PHONY: build test test-race test-coverage install clean lint fmt

build:
	go build -o push .

test:
	go test ./... -v

test-race:
	go test -race ./...

test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

install:
	go install .

clean:
	rm -f push coverage.out coverage.html
	go clean

lint:
	golangci-lint run

fmt:
	go fmt ./...
	goimports -w .

.DEFAULT_GOAL := build
