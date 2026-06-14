BINARY_NAME=dbinspect
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X github.com/Liyonglin20030201/task061402/cmd.Version=$(VERSION) -X github.com/Liyonglin20030201/task061402/cmd.BuildTime=$(BUILD_TIME)"

.PHONY: all build clean test test-cover vet lint

all: build

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) .

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe .

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

test:
	go test ./... -v

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

run:
	go run . $(ARGS)
