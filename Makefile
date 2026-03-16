.PHONY: lint test build coverage install-tools

build:
	go build -o aws-marketplace-cli .

lint:
	golangci-lint run ./...

test:
	go test -race ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
