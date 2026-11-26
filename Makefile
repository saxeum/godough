BINARY := bin/godough
LOAD_ENV := set -a; if [ -f .env ]; then . ./.env; fi; set +a

# Pinned, and run with `go run` so nothing has to be installed and go.mod is
# not touched. The vulnerability database updates itself.
VULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@v1.10.0
STATICCHECK := go run honnef.co/go/tools/cmd/staticcheck@v0.8.2

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

# Coverage. `make cover` reads a different number from `make test`, on purpose.
#
# -coverpkg=./... instruments every package in every test binary, so a line
# exercised by another package's test counts. Without it, internal/app reported
# New and Run at 0.0% while cmd/web's lifecycle test was running both.
#
# The generated contract is then filtered out: 613 lines nobody writes, with no
# branch in them that can be wrong. Leaving it in put the total at 68.0% instead
# of 85.8% and made the transport look a third worse than it is.
COVER_PROFILE  := coverage.out
COVER_FILTERED := coverage.filtered.out
COVER_GENERATED := internal/transport/httpapi/openapi.gen.go

# Every target says where it runs, in the text `make help` prints:
#
#   [host]    a process on your machine, reading .env
#   [docker]  a container, started by docker compose
#   [both]    a host process against the postgres container
#
# .env is read by host targets only. Compose sets DATABASE_URL itself, because
# the database is `postgres:5432` inside the network and 127.0.0.1:5432 outside.

.DEFAULT_GOAL := help
.PHONY: help build run ui-install ui-dev ui-build test test-unit test-race e2e cover cover-raw vet lint tls-check vuln vuln-images vuln-binary deps deps-image deps-build deps-graph why symbols fmt clean db-up stack-up down db-bootstrap db-migrate db-test test-db-reset codegen codegen-go codegen-ts codegen-check aws-test ecr-login ecr-build ecr-push ecr-pull-test

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
	$(LOAD_ENV); go run ./cmd/web

ui-install: ## [host] Install UI dependencies
	npm --prefix ui install

ui-dev: ## [host] Run the Vite dev server (:5173, proxies /api to :4000)
	npm --prefix ui run dev

ui-build: ## [host] Build the React UI into ui/dist
	npm --prefix ui run build

vet: ## [host] Run go vet
	go vet ./...

lint: ## [host] gofmt, staticcheck and oxlint
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt found unformatted files:"; echo "$$out"; exit 1; fi
	$(STATICCHECK) ./...
	npm --prefix ui run lint

tls-check: ## [both] Check Postgres CA and server certificate expiry
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run ./cmd/tlscheck -min-days $${TLS_MIN_DAYS:-30}

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

# The test database is disposable. Clear it before each run so a failed run
# cannot poison the next one. DELETE, not TRUNCATE: the application role has
# DELETE and does not own the table.
test-db-reset: ## [both] Clear the test database before a run
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; \
	: "$${TEST_DATABASE_URL:?TEST_DATABASE_URL is not set: copy .env.example to .env and run make db-test}"; \
	if [ -n "$$DATABASE_URL" ] && [ "$$TEST_DATABASE_URL" = "$$DATABASE_URL" ]; then \
		echo "TEST_DATABASE_URL must not equal DATABASE_URL: refusing to clear the development database"; \
		exit 1; \
	fi; \
	psql "$$TEST_DATABASE_URL" -v ON_ERROR_STOP=1 -c 'DELETE FROM sessions; DELETE FROM users; DELETE FROM snippets;' >/dev/null

test: test-db-reset ## [both] All tests against the test database (needs make db-test)
	$(LOAD_ENV); go test -count=1 -v -cover ./...

test-race: test-db-reset ## [both] All tests with the race detector
	$(LOAD_ENV); go test -race -count=1 -v ./...

cover: test-db-reset ## [both] HTML coverage report: cross-package counted, generated code excluded
	$(LOAD_ENV); go test -count=1 -coverpkg=./... -coverprofile=$(COVER_PROFILE) ./...
	@grep -v "$(COVER_GENERATED)" $(COVER_PROFILE) > $(COVER_FILTERED) || true
	@go tool cover -func=$(COVER_FILTERED) | tail -1
	go tool cover -html=$(COVER_FILTERED) -o coverage.html

cover-raw: cover ## [both] The same run, unfiltered: generated code included
	go tool cover -html=$(COVER_PROFILE) -o coverage-raw.html

# ---- infrastructure -----------------------------------------------------

AWS_REGION ?= ap-southeast-2
# Derived lazily, so `make help` and the non-ECR targets never call STS.
ECR_REGISTRY ?= $(shell aws sts get-caller-identity --query Account --output text 2>/dev/null | sed 's/$$/.dkr.ecr.$(AWS_REGION).amazonaws.com/')
ECR_DOCKER_CONFIG ?= $(CURDIR)/.docker-ecr
# TAG overrides both. Push defaults to HEAD; pull defaults to the last published
# master commit, so a pull-test on a feature branch does not ask ECR for an
# image that only exists in the working tree.
TAG ?=
PUSH_TAG ?= $(if $(TAG),$(TAG),$(shell git rev-parse HEAD))
PULL_TAG ?= $(if $(TAG),$(TAG),$(shell git rev-parse origin/master 2>/dev/null || git rev-parse HEAD))

aws-test: ## [host] Check the AWS state bucket exists and is not public
	cd infra/aws-test && go test -count=1 -v ./...

ecr-login: ## [host] Log Docker in to the ECR registry
	@test -n "$(ECR_REGISTRY)" || { echo "ECR_REGISTRY is not set: <account>.dkr.ecr.<region>.amazonaws.com"; exit 1; }
	@mkdir -p "$(ECR_DOCKER_CONFIG)"
	aws ecr get-login-password --region $(AWS_REGION) | DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker login --username AWS --password-stdin "$(ECR_REGISTRY)"

ecr-build: ## [host] Build api and web images tagged for ECR
	@test -n "$(ECR_REGISTRY)" || { echo "ECR_REGISTRY is not set: <account>.dkr.ecr.<region>.amazonaws.com"; exit 1; }
	@mkdir -p "$(ECR_DOCKER_CONFIG)"
	DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker build -t "$(ECR_REGISTRY)/godough-api:$(PUSH_TAG)" .
	DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker build -t "$(ECR_REGISTRY)/godough-web:$(PUSH_TAG)" ./ui

ecr-push: ecr-login ecr-build ## [host] Build and push api and web images by commit SHA
	DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker push "$(ECR_REGISTRY)/godough-api:$(PUSH_TAG)"
	DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker push "$(ECR_REGISTRY)/godough-web:$(PUSH_TAG)"

ecr-pull-test: ecr-login ## [host] Pull the ECR images and run compose plus e2e
	@test -n "$(ECR_REGISTRY)" || { echo "ECR_REGISTRY is not set: <account>.dkr.ecr.<region>.amazonaws.com"; exit 1; }
	DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker pull "$(ECR_REGISTRY)/godough-api:$(PULL_TAG)"
	DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker pull "$(ECR_REGISTRY)/godough-web:$(PULL_TAG)"
	ECR_REGISTRY="$(ECR_REGISTRY)" TAG="$(PULL_TAG)" docker compose -f docker-compose.yml -f docker-compose.ecr.yml up -d --wait --no-build
	$(MAKE) db-migrate
	$(MAKE) e2e

# ---- docker -------------------------------------------------------------

db-up: ## [docker] Start Postgres only (:5432)
	docker compose up -d --wait postgres

db-bootstrap: ## [docker] Create the application role and its grants (once per cluster)
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U godough -d snippet -1 < db/bootstrap.sql

db-migrate: ## [both] Apply pending migrations/*.sql (reads DATABASE_URL_OWNER)
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run ./cmd/migrate -dir migrations

db-test: db-up ## [both] Create, bootstrap and migrate the test database
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; \
	: "$${TEST_DATABASE_URL_OWNER:?TEST_DATABASE_URL_OWNER is not set: copy .env.example to .env}"; \
	if docker compose exec -T postgres psql -U godough -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='snippet_test'" | grep -qx 1; then \
		echo "snippet_test already exists"; \
	else \
		docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U godough -d postgres -c "CREATE DATABASE snippet_test"; \
	fi; \
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U godough -d snippet_test -1 < db/bootstrap.sql; \
	DATABASE_URL_OWNER="$$TEST_DATABASE_URL_OWNER" go run ./cmd/migrate -dir migrations

stack-up: ## [docker] Build and start postgres, api, web and the LB (:8080)
	docker compose up -d --wait --build

down: ## [docker] Stop all containers (keeps the volume; -v wipes it)
	docker compose down
