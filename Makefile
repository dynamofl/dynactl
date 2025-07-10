.PHONY: build test lint clean

# Build the binary
build: clean
	go build -o bin/dynactl ./cmd/dynactl

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build for multiple platforms
build-all: clean
	mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/dynactl-linux-amd64 ./cmd/dynactl
	GOOS=darwin GOARCH=amd64 go build -o bin/dynactl-darwin-amd64 ./cmd/dynactl
	GOOS=darwin GOARCH=arm64 go build -o bin/dynactl-darwin-arm64 ./cmd/dynactl
	GOOS=windows GOARCH=amd64 go build -o bin/dynactl-windows-amd64.exe ./cmd/dynactl

# Default target
all: deps lint test build 