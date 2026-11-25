BINARY := bin/godough

.DEFAULT_GOAL := help
.PHONY: help build run test cover vet fmt clean

help: ## Show this help
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-6s\033[0m %s\n", $$1, $$2}'

build: ## Compile to bin/godough
	go build -o $(BINARY) .

run: ## Run the server (listens on :8080)
	go run .

test: ## Run the tests, verbose, with coverage
	go test -v -cover ./...

cover: ## Write an HTML coverage report to coverage.html
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -l -w .

clean: ## Remove build output and coverage reports
	rm -rf bin coverage.out coverage.html
