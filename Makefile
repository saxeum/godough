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
.PHONY: help build run ui-install ui-dev ui-build test test-unit test-race e2e cover cover-raw vet lint tls-check vuln vuln-images vuln-binary deps deps-image deps-build deps-graph why symbols fmt clean db-up stack-up down db-bootstrap db-migrate db-test test-db-reset rds-app-password rds-bootstrap codegen codegen-go codegen-ts codegen-check aws-test ecr-login ecr-build ecr-push ecr-pull-test rds-tunnel rds-psql k8s-secrets eks-tunnel eks-kubeconfig charts-lint image-digests check-argocd argocd-verify argocd-prune argocd-seed argocd-forward argocd-password argocd-login argocd-apps argocd-diff

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
TF_DATA := infra/terraform/data
TF_EKS := infra/terraform/eks
# The standard kubeconfig, because `aws eks update-kubeconfig` merges a context
# rather than replacing the file: kubectl then works with nothing exported. Set
# EKS_KUBECONFIG to a path to keep it out of ~/.kube entirely.
EKS_KUBECONFIG ?= $(HOME)/.kube/config
PSQL_ARGS ?=
# Derived lazily, so `make help` and the non-ECR targets never call STS.
ECR_REGISTRY ?= $(shell aws sts get-caller-identity --query Account --output text 2>/dev/null | sed 's/$$/.dkr.ecr.$(AWS_REGION).amazonaws.com/')
ECR_DOCKER_CONFIG ?= $(CURDIR)/.docker-ecr
# TAG overrides both. Push defaults to HEAD; pull defaults to the last published
# master commit, so a pull-test on a feature branch does not ask ECR for an
# image that only exists in the working tree.
TAG ?=
PUSH_TAG ?= $(if $(TAG),$(TAG),$(shell git rev-parse HEAD))
PULL_TAG ?= $(if $(TAG),$(TAG),$(shell git rev-parse origin/master 2>/dev/null || git rev-parse HEAD))

rds-tunnel: ## [host] Forward RDS 5432 to localhost:15432 through SSM
	@command -v session-manager-plugin >/dev/null || { echo "session-manager-plugin is not installed"; exit 1; }
	@target=$$(terraform -chdir=$(TF_DATA) output -raw bastion_instance_id); \
	 endpoint=$$(terraform -chdir=$(TF_DATA) output -raw rds_endpoint | cut -d: -f1); \
	 aws ssm start-session --region $(AWS_REGION) --target "$$target" --document-name AWS-StartPortForwardingSessionToRemoteHost --parameters host="$$endpoint",portNumber="5432",localPortNumber="15432"

rds-psql: ## [host] psql to RDS through the running rds-tunnel
	@test -f global-bundle.pem || { echo "download global-bundle.pem first: https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem"; exit 1; }
	@secret=$$(terraform -chdir=$(TF_DATA) output -raw master_user_secret_arn); \
	 password=$$(aws secretsmanager get-secret-value --region $(AWS_REGION) --secret-id "$$secret" --query SecretString --output text | jq -r .password); \
	 endpoint=$$(terraform -chdir=$(TF_DATA) output -raw rds_endpoint | cut -d: -f1); \
	 PGPASSWORD="$$password" psql "host=$$endpoint hostaddr=127.0.0.1 port=15432 dbname=godough user=godough sslmode=verify-full sslrootcert=global-bundle.pem" $(PSQL_ARGS)

# Pinned like the Trivy image, and run through Docker so the host needs no Helm.
HELM := docker run --rm -v $(CURDIR):/apps -w /apps alpine/helm:4.0.0
CHARTS := deploy/charts/api deploy/charts/ui
# A placeholder digest: 64 hex characters, so the rendered manifests are the
# shape a real one produces. The charts refuse to render without one.
CHART_RENDER_ARGS := --set image.repository=registry.example/godough --set image.digest=sha256:0000000000000000000000000000000000000000000000000000000000000000

charts-lint: ## [host] Lint and render both charts, and check the digest guard
	@for c in $(CHARTS); do \
	  $(HELM) lint $$c $(CHART_RENDER_ARGS) >/dev/null || exit 1; \
	  $(HELM) template $$(basename $$c) $$c $(CHART_RENDER_ARGS) >/dev/null || exit 1; \
	  echo "ok $$c"; \
	done
	@# A chart that renders without a digest deploys a moving tag, so the guard
	@# is part of the gate rather than a nicety.
	@if $(HELM) template no-digest $(firstword $(CHARTS)) --set image.repository=registry.example/godough >/dev/null 2>&1; then \
	  echo "a chart rendered without image.digest: the guard is gone"; exit 1; \
	fi
	@echo "ok the digest guard still fails without a digest"

eks-tunnel: ## [host] Forward the private EKS API to localhost:6443 through SSM
	@command -v session-manager-plugin >/dev/null || { echo "session-manager-plugin is not installed"; exit 1; }
	@target=$$(terraform -chdir=$(TF_DATA) output -raw bastion_instance_id); \
	 host=$$(terraform -chdir=$(TF_EKS) output -raw cluster_endpoint | sed 's|https://||'); \
	 echo "forwarding $$host:443 to 127.0.0.1:6443 through $$target; leave this running, ^C to stop"; \
	 aws ssm start-session --region $(AWS_REGION) --target "$$target" --document-name AWS-StartPortForwardingSessionToRemoteHost --parameters host="$$host",portNumber="443",localPortNumber="6443"

image-digests: ## [host] Print the newest published tag and image digests
	@for repo in godough-api godough-web; do \
	  aws ecr describe-images --region $(AWS_REGION) --repository-name $$repo \
	    --query 'reverse(sort_by(imageDetails[?imageTags],&imagePushedAt))[0].[imageTags[0],imageDigest]' --output text \
	    | awk -v r="$$repo" '{printf "%-14s %-42s %s\n", r, $$1, $$2}'; \
	done
	@echo "newest published, which is not necessarily origin/master: a merge that publishes nothing leaves no image"

eks-kubeconfig: ## [host] Point kubectl at the tunnel above, as context godough-staging
	@endpoint=$$(terraform -chdir=$(TF_EKS) output -raw cluster_endpoint); \
	 host=$${endpoint#https://}; \
	 aws eks update-kubeconfig --region $(AWS_REGION) --kubeconfig $(EKS_KUBECONFIG) \
	   --name "$$(terraform -chdir=$(TF_EKS) output -raw cluster_name)" --alias godough-staging >/dev/null || exit 1; \
	 cluster=$$(kubectl config view --kubeconfig $(EKS_KUBECONFIG) \
	   -o jsonpath="{.contexts[?(@.name=='godough-staging')].context.cluster}"); \
	 kubectl config set-cluster "$$cluster" --kubeconfig $(EKS_KUBECONFIG) \
	   --server=https://127.0.0.1:6443 --tls-server-name="$$host" >/dev/null || exit 1; \
	 kubectl config use-context godough-staging --kubeconfig $(EKS_KUBECONFIG) >/dev/null || exit 1; \
	 echo "context godough-staging in $(EKS_KUBECONFIG) dials 127.0.0.1:6443 as $$host"; \
	 if [ -n "$${KUBECONFIG:-}" ] && [ "$$KUBECONFIG" != "$(EKS_KUBECONFIG)" ]; then \
	   echo "warning: this shell exports KUBECONFIG=$$KUBECONFIG, which overrides that file"; \
	 elif kubectl version --request-timeout=3s >/dev/null 2>&1; then \
	   echo "server: reachable"; \
	 else \
	   echo "server: NOT reachable. The endpoint is private: run 'make eks-tunnel' in another terminal."; \
	 fi

# ---- Argo CD -------------------------------------------------------------
#
# Argo has no ingress on purpose: an exposed control plane is a much larger
# surface than a port-forward. The forward still needs the EKS API, which is
# private, so the tunnel comes first — every target here says so rather than
# hanging.

ARGOCD_NAMESPACE ?= argocd
# 8090, not 8080: the compose stack's load balancer already owns 8080 locally.
ARGOCD_ADDR ?= 127.0.0.1:8090

argocd-forward: ## [host] Forward the Argo CD UI and API to 127.0.0.1:8090 (needs make eks-tunnel)
	@kubectl version --request-timeout=3s >/dev/null 2>&1 || { echo "cannot reach the cluster: run 'make eks-tunnel' in another terminal"; exit 1; }
	@echo "http://$(ARGOCD_ADDR)  —  user admin, password from 'make argocd-password'"
	@kubectl -n $(ARGOCD_NAMESPACE) port-forward svc/argocd-server 8090:80

argocd-password: ## [host] Print the generated admin password, until someone changes it
	@kubectl version --request-timeout=3s >/dev/null 2>&1 || { echo "cannot reach the cluster: run 'make eks-tunnel' in another terminal"; exit 1; }
	@pw=$$(kubectl -n $(ARGOCD_NAMESPACE) get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' 2>/dev/null | base64 -d 2>/dev/null); \
	 [ -n "$$pw" ] || { echo "no argocd-initial-admin-secret: the password was changed, or Argo was reinstalled without it"; exit 1; }; \
	 echo "$$pw"

argocd-login: ## [host] Log the argocd CLI in to the forwarded server (needs make argocd-forward)
	@command -v argocd >/dev/null || { echo "argocd CLI not installed: https://argo-cd.readthedocs.io/en/stable/cli_installation/"; exit 1; }
	@echo "password: $$(kubectl -n $(ARGOCD_NAMESPACE) get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d)"
	@# The password is printed and pasted, not passed in argv: an argument is visible
	@# in `ps` to every user on the machine, and this one is a bootstrap credential.
	@argocd login $(ARGOCD_ADDR) --insecure --username admin

argocd-apps: ## [host] Applications with their sync state and health (needs make eks-tunnel)
	@kubectl version --request-timeout=3s >/dev/null 2>&1 || { echo "cannot reach the cluster: run 'make eks-tunnel' in another terminal"; exit 1; }
	@kubectl -n $(ARGOCD_NAMESPACE) get applications \
	  -o custom-columns='NAME:.metadata.name,SYNC:.status.sync.status,HEALTH:.status.health.status'

argocd-prune: ## [host] Sync one Application WITH prune: make argocd-prune APP=godough-api (needs argocd-forward and argocd-login)
	@test -n "$(APP)" || { echo "set APP: make argocd-prune APP=godough-api"; exit 1; }
	@command -v argocd >/dev/null || { echo "argocd CLI not installed"; exit 1; }
	@echo "this deletes objects that are in the cluster but not in git, for $(APP)"
	@# Prune is off in the Application on purpose: an object that stops rendering
	@# should not disappear because a values key was mistyped, and nothing runs the
	@# verifier on a loop. This is the deliberate version of that act, one app at a
	@# time, with the diff available first through `make argocd-diff`.
	@argocd app sync $(APP) --prune --server $(ARGOCD_ADDR) --insecure

argocd-verify: ## [host] Assert every Application is Synced/Healthy and every pod runs the digest the values file names (needs make eks-tunnel)
	./deploy/argocd/verify.sh

check-argocd: ## [host] Render the pinned Argo CD chart with its values, and check the pin agrees with Terraform
	./deploy/argocd/install/check.sh

argocd-seed: ## [host] Apply the root Application, once: the app-of-apps bootstrap (needs make eks-tunnel)
	@kubectl version --request-timeout=3s >/dev/null 2>&1 || { echo "cannot reach the cluster: run 'make eks-tunnel' in another terminal"; exit 1; }
	@# One object is hand-placed and everything below it arrives through sync. This is
	@# the documented bootstrap exception, and it is idempotent: re-running it after an
	@# edit is the same as letting Argo notice.
	@kubectl apply -f deploy/argocd/root/
	@echo "root applied; watch it with 'make argocd-apps'"

argocd-diff: ## [host] Diff one Application against git: make argocd-diff APP=godough-api (needs argocd-forward and argocd-login)
	@test -n "$(APP)" || { echo "set APP: make argocd-diff APP=godough-api"; exit 1; }
	@command -v argocd >/dev/null || { echo "argocd CLI not installed"; exit 1; }
	@argocd app diff $(APP) --server $(ARGOCD_ADDR) --insecure

aws-test: ## [host] Check the AWS state bucket exists and is not public
	cd infra/aws-test && go test -count=1 -v ./...

ecr-login: ## [host] Log Docker in to the ECR registry
	@test -n "$(ECR_REGISTRY)" || { echo "ECR_REGISTRY is not set: <account>.dkr.ecr.<region>.amazonaws.com"; exit 1; }
	@mkdir -p "$(ECR_DOCKER_CONFIG)"
	aws ecr get-login-password --region $(AWS_REGION) | DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker login --username AWS --password-stdin "$(ECR_REGISTRY)"

ecr-build: ## [host] Build api and web images tagged for ECR
	@test -n "$(ECR_REGISTRY)" || { echo "ECR_REGISTRY is not set: <account>.dkr.ecr.<region>.amazonaws.com"; exit 1; }
	@mkdir -p "$(ECR_DOCKER_CONFIG)"
# --platform is stated rather than inherited: the cluster's nodes are x86_64, and
# an image built on an ARM machine or with a buildx default would publish
# something the kubelet cannot run (see #134).
	DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker build --platform linux/amd64 -t "$(ECR_REGISTRY)/godough-api:$(PUSH_TAG)" .
	DOCKER_CONFIG="$(ECR_DOCKER_CONFIG)" docker build --platform linux/amd64 -t "$(ECR_REGISTRY)/godough-web:$(PUSH_TAG)" ./ui

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

# The application role's password: its own secret, separate from the RDS-managed
# master secret, and created by a human rather than by Terraform so it is not in
# state, a plan file, or this repository (#114). Re-running changes nothing; a
# rotation is an explicit put-secret-value, not a side effect.
APP_PASSWORD_SECRET ?= godough-staging/app-password

rds-app-password: ## [host] Create the godough_app password in Secrets Manager (once)
	@if aws secretsmanager describe-secret --region $(AWS_REGION) --secret-id $(APP_PASSWORD_SECRET) >/dev/null 2>&1; then \
	  echo "$(APP_PASSWORD_SECRET) already exists; leaving it alone"; \
	else \
	  aws secretsmanager create-secret --region $(AWS_REGION) --name $(APP_PASSWORD_SECRET) \
	    --description "godough_app password for the staging database" \
	    --secret-string "{\"password\":\"$$(openssl rand -hex 24)\"}" >/dev/null; \
	  echo "created $(APP_PASSWORD_SECRET)"; \
	fi; \
	echo "the value is never printed: read it with aws secretsmanager get-secret-value"

# Runs as the master, through the tunnel, because the role is cluster-scoped and
# only the owner may create it. The password comes from Secrets Manager, not from
# this file.
# The two Secrets the api chart needs, created from Secrets Manager and the CA
# bundle. Imperative on purpose: External Secrets or the Secrets Store CSI driver
# would sync these automatically, and both cost an operator, a CRD and an IRSA
# role. With one consumer and one operator, a documented command that has to be
# re-run after a rotation is the smaller thing; when a second workload needs the
# same credential, the operator starts to pay for itself.
K8S_NAMESPACE ?= godough
# What Google will redirect back to. Must match the authorised redirect URI in the
# Google console exactly, or the callback fails after the user has already
# consented — which reads as a sign-in bug rather than a configuration mismatch.
EDGE_URL ?= https://godough.foonix.net

k8s-secrets: ## [host] Create the DSN, CA and sign-in Secrets in the cluster (needs the EKS tunnel)
	@test -f global-bundle.pem || { echo "download global-bundle.pem first: https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem"; exit 1; }
	@kubectl version --request-timeout=3s >/dev/null 2>&1 || { echo "cannot reach the cluster: run 'make eks-tunnel' in another terminal"; exit 1; }
	@kubectl create namespace $(K8S_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f - >/dev/null
	@app=$$(aws secretsmanager get-secret-value --region $(AWS_REGION) --secret-id $(APP_PASSWORD_SECRET) --query SecretString --output text | jq -r .password); \
	 endpoint=$$(terraform -chdir=$(TF_DATA) output -raw rds_endpoint | cut -d: -f1); \
	 dsn="postgres://godough_app:$$app@$$endpoint/godough?sslmode=verify-full&sslrootcert=/certs/global-bundle.pem&pool_max_conns=5"; \
	 kubectl -n $(K8S_NAMESPACE) create secret generic godough-api \
	   --from-literal=DATABASE_URL="$$dsn" --dry-run=client -o yaml | kubectl apply -f - ; \
	 kubectl -n $(K8S_NAMESPACE) create secret generic godough-rds-ca \
	   --from-file=global-bundle.pem=./global-bundle.pem --dry-run=client -o yaml | kubectl apply -f - ; \
	 master_secret=$$(terraform -chdir=$(TF_DATA) output -raw master_user_secret_arn); \
	 master=$$(aws secretsmanager get-secret-value --region $(AWS_REGION) --secret-id "$$master_secret" --query SecretString --output text); \
	 m_user=$$(echo "$$master" | jq -r .username); \
	 m_pw=$$(echo "$$master" | jq -r .password); \
	 owner_dsn="host=$$endpoint dbname=godough user=$$m_user password='$$m_pw' sslmode=verify-full sslrootcert=/certs/global-bundle.pem"; \
	 kubectl -n $(K8S_NAMESPACE) create secret generic godough-owner \
	   --from-literal=DATABASE_URL_OWNER="$$owner_dsn" --dry-run=client -o yaml | kubectl apply -f - ; \
	 if [ -n "$$GOOGLE_CLIENT_ID" ] && [ -n "$$GOOGLE_CLIENT_SECRET" ]; then \
	   kubectl -n $(K8S_NAMESPACE) create secret generic godough-oauth \
	     --from-literal=GOOGLE_CLIENT_ID="$$GOOGLE_CLIENT_ID" \
	     --from-literal=GOOGLE_CLIENT_SECRET="$$GOOGLE_CLIENT_SECRET" \
	     --from-literal=GOOGLE_REDIRECT_URL="$(EDGE_URL)/api/auth/google/callback" \
	     --dry-run=client -o yaml | kubectl apply -f - ; \
	   echo "godough-oauth is in $(K8S_NAMESPACE), redirecting to $(EDGE_URL)/api/auth/google/callback"; \
	 else \
	   echo "GOOGLE_CLIENT_ID/SECRET are empty: sign-in stays disabled"; \
	 fi; \
	 echo "godough-api, godough-rds-ca and godough-owner are in $(K8S_NAMESPACE); re-run this after rotating a password"
	@# Keyword/value, not a URL: a generated RDS password containing @ or / is
	@# silently mangled by URL parsing, and the failure looks like bad credentials.

rds-bootstrap: ## [host] Create godough_app on RDS from its Secrets Manager password
	@test -f global-bundle.pem || { echo "download global-bundle.pem first: https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem"; exit 1; }
	@pg_isready -h 127.0.0.1 -p 15432 -q || { echo "nothing is listening on 15432: run 'make rds-tunnel' in another terminal"; exit 1; }
	@secret=$$(terraform -chdir=$(TF_DATA) output -raw master_user_secret_arn); \
	 master=$$(aws secretsmanager get-secret-value --region $(AWS_REGION) --secret-id "$$secret" --query SecretString --output text | jq -r .password); \
	 app=$$(aws secretsmanager get-secret-value --region $(AWS_REGION) --secret-id $(APP_PASSWORD_SECRET) --query SecretString --output text | jq -r .password); \
	 endpoint=$$(terraform -chdir=$(TF_DATA) output -raw rds_endpoint | cut -d: -f1); \
	 echo "bootstrapping godough_app on $$endpoint through the tunnel on :15432"; \
	 PGPASSWORD="$$master" psql "host=$$endpoint hostaddr=127.0.0.1 port=15432 dbname=godough user=godough sslmode=verify-full sslrootcert=global-bundle.pem" \
	   -v ON_ERROR_STOP=1 -v app_password="$$app" -1 -f db/bootstrap.sql

db-bootstrap: ## [docker] Create the application role and its grants (once per cluster)
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U godough -d snippet -1 < db/bootstrap.sql

db-migrate: ## [both] Apply pending migrations/*.sql (reads DATABASE_URL_OWNER)
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run ./cmd/migrate

db-test: db-up ## [both] Create, bootstrap and migrate the test database
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; \
	: "$${TEST_DATABASE_URL_OWNER:?TEST_DATABASE_URL_OWNER is not set: copy .env.example to .env}"; \
	if docker compose exec -T postgres psql -U godough -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='snippet_test'" | grep -qx 1; then \
		echo "snippet_test already exists"; \
	else \
		docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U godough -d postgres -c "CREATE DATABASE snippet_test"; \
	fi; \
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U godough -d snippet_test -1 < db/bootstrap.sql; \
	DATABASE_URL_OWNER="$$TEST_DATABASE_URL_OWNER" go run ./cmd/migrate

stack-up: ## [docker] Build and start postgres, api, web and the LB (:8080)
	docker compose up -d --wait --build

down: ## [docker] Stop all containers (keeps the volume; -v wipes it)
	docker compose down
