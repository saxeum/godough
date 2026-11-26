BINARY := bin/godough
LOAD_ENV := set -a && . ./.env && set +a

# Pinned, and run with `go run` so nothing has to be installed and go.mod is
# not touched. The vulnerability database updates itself.
VULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@v1.10.0

# Trivy through docker: no install, and the same binary locally and in CI.
TRIVY := docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.76.2 image --scanners vuln --quiet

# The contract both languages are generated from, and the pinned generators.
# Same policy as govulncheck: `go run` / npx with an exact version, so nothing
# has to be installed and go.mod stays a description of the compiled binary.
SPEC := spec/openapi.yaml
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.9.0
GENERATED := internal/transport/httpapi/openapi.gen.go ui/src/api/generated

# How many modules to show in `make symbols`.
N ?= 20

# Every target says where it runs, in the text `make help` prints:
#
#   [host]    a process on your machine, reading .env
#   [docker]  a container, started by docker compose
#   [both]    a host process against the postgres container
#
# .env is read by host targets only. Compose sets DATABASE_URL itself, because
# the database is `postgres:5432` inside the network and 127.0.0.1:5432 outside.

.DEFAULT_GOAL := help
.PHONY: help build run ui-install ui-dev ui-build test test-unit e2e cover vet vuln vuln-images vuln-binary deps deps-image deps-build deps-graph why symbols fmt clean db-up stack-up down db-migrate codegen codegen-go codegen-ts codegen-check

help: ## Show this help
	@grep -hE '^[a-z0-9-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

# ---- the contract --------------------------------------------------------

codegen-go: ## [host] Regenerate the Go server contract from spec/openapi.yaml
	$(OAPI_CODEGEN) -config spec/oapi-codegen.yaml $(SPEC)

codegen-ts: ## [host] Regenerate the UI's types and client from spec/openapi.yaml
	npm --prefix ui run codegen

codegen: codegen-go codegen-ts ## [host] Regenerate both sides

# The gate: regenerate, then fail if the tree changed. `git status --porcelain`
# rather than `git diff --exit-code` because a newly generated file is untracked,
# and an untracked file is exactly the drift this is looking for.
codegen-check: ## [host] Fail if the committed generated code is stale
	@$(MAKE) --no-print-directory -s codegen
	@out=$$(git status --porcelain -- $(GENERATED)); \
	if [ -n "$$out" ]; then \
		echo "generated code is out of date; run make codegen and commit the result"; \
		echo "$$out"; \
		exit 1; \
	fi

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

deps: ## [host] Modules linked into bin/godough, from its buildinfo
	@test -x $(BINARY) || $(MAKE) --no-print-directory -s build
	@go version -m $(BINARY) | awk '$$1 == "dep" {printf "  %-58s %s\n", $$2, $$3}'

deps-image: ## [docker] The same list, read from the binary inside the api image
	@cid=$$(docker create godough-api) && \
	trap 'docker rm $$cid >/dev/null' EXIT && \
	docker cp $$cid:/godough /tmp/godough-image-$$$$ >/dev/null && \
	trap 'rm -f /tmp/godough-image-$$$$; docker rm $$cid >/dev/null' EXIT && \
	go version -m /tmp/godough-image-$$$$ | awk '$$1 == "dep" {printf "  %-58s %s\n", $$2, $$3}'

deps-build: ## [host] Modules whose packages the build needs (go list -deps)
	@go list -deps -f '{{with .Module}}{{.Path}} {{.Version}}{{end}}' ./cmd/web | grep -v '^$$' | sort -u

deps-graph: ## [host] Every module in the module graph, test-only ones included
	@go list -m all

why: ## [host] Why a module is in the graph at all: make why MOD=<module>
	@test -n "$(MOD)" || { echo "usage: make why MOD=<module>"; exit 1; }
	@go mod why -m $(MOD)

symbols: ## [host] Function symbols per module in bin/godough (needs no strip)
	@test -x $(BINARY) || $(MAKE) --no-print-directory -s build
	@go tool nm $(BINARY) | awk '{print $$NF}' \
		| grep -vE '(^|/)vendor/' \
		| grep -oE '(github\.com|golang\.org|gopkg\.in)/[^/]+/[^/.]+' \
		| sort | uniq -c | sort -rn | head -$(N)

vuln-images: ## [docker] Fixable HIGH/CRITICAL in both images (builds them first)
	@docker compose build --quiet api web
	@for img in godough-api godough-web; do \
		printf '%s: ' "$$img"; \
		if out=$$($(TRIVY) --ignore-unfixed --severity HIGH,CRITICAL --exit-code 1 "$$img" 2>&1); then \
			echo ok; \
		else \
			echo FAILED; echo "$$out"; exit 1; \
		fi; \
	done

vuln: ## [host] Known vulnerabilities reachable from this code (govulncheck)
	$(VULNCHECK) ./...

vuln-binary: ## [host] The same scan on bin/godough, as the image ships it
	$(VULNCHECK) -mode=binary $(BINARY)

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
