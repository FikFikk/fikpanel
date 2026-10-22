BINARY_NAME=fik-panel
BUILD_DIR=bin
VERSION?=0.1.0-dev

.PHONY: all build build-linux test clean

all: build

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/fik-panel

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/fik-panel

test:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)
