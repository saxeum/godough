BINARY := bin/godough
LOAD_ENV := set -a && . ./.env && set +a

.DEFAULT_GOAL := help
.PHONY: help build run db-up db-down db-migrate ui-install ui-dev ui-build test cover vet fmt clean

help: ## Show this help
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-11s\033[0m %s\n", $$1, $$2}'

build: ## Compile the server to bin/godough
	go build -o $(BINARY) ./cmd/web

run: ## Run the server, using .env for SNIPPET_ADDR and DATABASE_URL
	$(LOAD_ENV) && go run ./cmd/web

db-up: ## Start Postgres (docker compose, :5432)
	docker compose up -d --wait

db-down: ## Stop Postgres (keeps the volume; -v wipes it)
	docker compose down

db-migrate: ## Apply migrations/*.sql to the dev database
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U godough -d snippet -1 < migrations/0001_create_snippets.sql

ui-install: ## Install UI dependencies
	npm --prefix ui install

ui-dev: ## Run the Vite dev server (:5173, proxies /snippet to :4000)
	npm --prefix ui run dev

ui-build: ## Build the React UI into ui/dist
	npm --prefix ui run build

test: ## Run the tests, verbose, with coverage (needs .env and a migrated DB)
	$(LOAD_ENV) && go test -v -cover ./...

cover: ## Write an HTML coverage report to coverage.html
	$(LOAD_ENV) && go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -l -w .

clean: ## Remove build output, coverage and UI build
	rm -rf bin coverage.out coverage.html ui/dist
