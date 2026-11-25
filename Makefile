BINARY := bin/godough
LOAD_ENV := set -a && . ./.env && set +a

# Every target says where it runs, in the text `make help` prints:
#
#   [host]    a process on your machine, reading .env
#   [docker]  a container, started by docker compose
#   [both]    a host process against the postgres container
#
# .env is read by host targets only. Compose sets DATABASE_URL itself, because
# the database is `postgres:5432` inside the network and 127.0.0.1:5432 outside.

.DEFAULT_GOAL := help
.PHONY: help build run ui-install ui-dev ui-build test test-unit e2e cover vet fmt clean db-up stack-up down db-migrate

help: ## Show this help
	@grep -hE '^[a-z0-9-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

# ---- host ---------------------------------------------------------------

build: ## [host] Compile the API to bin/godough
	go build -o $(BINARY) ./cmd/web

run: ## [host] Run the API only (:4000, .env; the UI is make ui-dev)
	$(LOAD_ENV) && go run ./cmd/web

ui-install: ## [host] Install UI dependencies
	npm --prefix ui install

ui-dev: ## [host] Run the Vite dev server (:5173, proxies /api to :4000)
	npm --prefix ui run dev

ui-build: ## [host] Build the React UI into ui/dist
	npm --prefix ui run build

vet: ## [host] Run go vet
	go vet ./...

fmt: ## [host] Format all Go files
	gofmt -l -w .

clean: ## [host] Remove build output, coverage and UI build
	rm -rf bin coverage.out coverage.html ui/dist

# ---- host against the postgres container --------------------------------

e2e: ## [docker] End-to-end checks against the running stack (:8080)
	BASE_URL=$${BASE_URL:-http://localhost:8080} scripts/e2e.sh

test-unit: ## [host] Fast tests, no database, no .env (-short)
	go test -count=1 -short -v ./...

test: ## [both] All tests, including the database (needs make db-up db-migrate)
	$(LOAD_ENV) && go test -count=1 -v -cover ./...

cover: ## [both] HTML coverage report from the full test run
	$(LOAD_ENV) && go test -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# ---- docker -------------------------------------------------------------

db-up: ## [docker] Start Postgres only (:5432)
	docker compose up -d --wait postgres

db-migrate: ## [docker] Apply migrations/*.sql to the dev database
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U godough -d snippet -1 < migrations/0001_create_snippets.sql

stack-up: ## [docker] Build and start postgres, api, web and the LB (:8080)
	docker compose up -d --wait --build

down: ## [docker] Stop all containers (keeps the volume; -v wipes it)
	docker compose down
