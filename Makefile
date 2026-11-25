BINARY := bin/godough

.DEFAULT_GOAL := help
.PHONY: help build run ui-install ui-dev ui-build test cover vet fmt clean

help: ## Show this help
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-11s\033[0m %s\n", $$1, $$2}'

build: ## Compile the server to bin/godough
	go build -o $(BINARY) ./cmd/web

run: ## Run the server (:4000, serving ui/dist)
	go run ./cmd/web

ui-install: ## Install UI dependencies
	npm --prefix ui install

ui-dev: ## Run the Vite dev server (:5173, proxies /snippet to :4000)
	npm --prefix ui run dev

ui-build: ## Build the React UI into ui/dist
	npm --prefix ui run build

test: ## Run the tests, verbose, with coverage
	go test -v -cover ./...

cover: ## Write an HTML coverage report to coverage.html
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -l -w .

clean: ## Remove build output, coverage and UI build
	rm -rf bin coverage.out coverage.html ui/dist
