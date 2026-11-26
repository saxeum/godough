# godough

A snippet manager: a Go API over Postgres, a React UI, and an nginx load
balancer in front of both. It started as a rewrite of the Snippetbox exercise
with pgx and Postgres instead of `database/sql` and MySQL.

## Running it

```
cp .env.example .env
make db-up          # postgres only
make db-migrate     # apply migrations/*.sql
make stack-up       # build and start postgres, api, web and the LB
```

Then <http://localhost:8080> serves the app, signed out until Google
credentials are configured. `make down` stops the containers and keeps the
database volume; `docker compose down -v` wipes it.

Four containers, one routing rule — `/api/*` goes to the api container,
everything else to the web container:

```
       :8080
         |
       nginx  (nginx/lb.conf)
       /    \
    api     web
   :4000    :8080   (internal)
      |
  postgres :5432
```

The API owns the `/api` prefix and the LB does not rewrite it, so the same rule
can be expressed by an ingress later without touching the API. Because
everything is one origin, there is no CORS anywhere, and the UI needs no API
base URL baked in at build time.

## Developing

Two host processes against the containerised database:

```
make db-up
make run            # API on :4000, reads .env
make ui-dev         # Vite on :5173, proxies /api to :4000
```

Every `make` target says where it runs, in the text `make help` prints:

- `[host]` — a process on your machine, reading `.env`
- `[docker]` — a container, started by docker compose
- `[both]` — a host process against the postgres container

`.env` is read by host targets only. Compose sets `DATABASE_URL` and
`TRUSTED_ORIGINS` itself, because the database and the public origin are
different inside the network than outside it.

- `SNIPPET_ADDR` — host:port the API listens on (`:4000`)
- `LOG_LEVEL` — `debug`, `info`, `warn` or `error`; defaults to `info`
- `LOG_FORMAT` — `text` or `json`; defaults to `json`. `.env` sets `text` for a
  host process; containers keep the JSON default
- `TRUSTED_ORIGINS` — comma-separated exact origins allowed to make
  cookie-authenticated writes. Empty fails closed; a malformed value refuses to
  boot
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL` — set all
  three or none. Without them `/api/auth/google` is `503`; the secret is a
  deployment secret, never an image layer. The redirect URI must be registered
  on the OAuth client; the local one is
  `http://localhost:8080/api/auth/google/callback`
- `DATABASE_URL` — a pgx DSN; the app connects as `godough_app`, a non-superuser
  that owns nothing, created by `db/bootstrap.sql` (`make db-bootstrap`).
  Outside the compose network it uses `sslmode=verify-full` and an
  `sslrootcert` CA bundle
- `DATABASE_URL_OWNER` — the owner's DSN, read only by `make db-migrate`. Two
  variables rather than one so the migrator cannot accidentally run as the app
  role, which would fail on the first `CREATE TABLE`
- `TEST_DATABASE_URL` — the application role's DSN for `snippet_test`;
  `make test` refuses to run without it
- `TEST_DATABASE_URL_OWNER` — the owner's DSN for `make db-test`
- Migrations run as `godough`, the owner, and record themselves in
  `schema_migrations`

The two runtime variables (`SNIPPET_ADDR`, `DATABASE_URL`) are required. The
test variables are required by `make db-test` and `make test`. A missing runtime
value exits at startup with its name; a bad `LOG_LEVEL` or `LOG_FORMAT` exits
with the offending value in the error.

`.env.example` shows the production TLS form: `verify-full` plus a CA bundle.
The local compose Postgres has no certificate, so a host process pointed at that
container needs `sslmode=disable` in its own `.env`; the api container is
already configured that way. A missing CA is a startup failure, not a fallback
to plaintext. In deployment the CA bundle is a mounted Secret, and rotating it
is a restart, not a hot reload.

`make tls-check` reads `DATABASE_URL`, prints every certificate in `sslrootcert`
and the server's peer certificate, and exits non-zero when anything expires
inside `TLS_MIN_DAYS` (default 30). It skips `sslmode=disable` so the local
compose stack remains usable. In #5 this is the command to wire as a CronJob or
alert, not a check that waits for the first failed connection.

Every request gets one structured line: `request_id`, `method`, `route`,
`status`, `duration` and `bytes`. A safe incoming `X-Request-ID` is preserved
and echoed; otherwise one is generated. The same id is on the response and on
the error line for a `5xx`.

## Contract

`spec/openapi.yaml` is the only description of the wire format. Both languages
are generated from it and neither generated file is edited by hand:

```
make codegen        # both
make codegen-go     # internal/transport/httpapi/openapi.gen.go, via oapi-codegen
make codegen-ts     # ui/src/api/generated, via orval
make codegen-check  # regenerate, then fail if the tree changed
```

`codegen-check` is a required CI check, so a spec change that nobody regenerated
lands there rather than in production. The output is committed because the web
image builds from `ui/` alone and cannot see a spec at the repository root — and
because a drift gate needs a committed copy to diff against.

The Go side is generated with `strict-server` and `std-http-server`, which is
what makes the layering real: the controller implements an interface whose
methods take typed request objects and return typed response objects, so a
documented status code that no branch produces does not compile. The TypeScript
side generates one function per operation over a custom `fetch` mutator, so a
failure still arrives as an `ApiError` with its per-field messages.

## Layout

```
cmd/web/                       main: config, then run
cmd/migrate/                   applies pending migrations; ships in the same image
internal/migrate/              discovery, ordering and the per-file transaction
internal/config/               environment -> Config, validated at startup
internal/domain/snippet/       entity, create-input, ErrNotFound, ValidationError
internal/service/              the rules; declares the repository interface it uses
internal/repository/postgres/  the only package that knows SQL
internal/transport/httpapi/    generated contract + controller + probes + routes
internal/app/                  wiring and the process lifecycle
spec/openapi.yaml              the contract
spec/oapi-codegen.yaml         its Go generator configuration
ui/orval.config.ts             its TypeScript generator configuration
migrations/                    plain SQL, applied by make db-migrate
db/bootstrap.sql               the application role and its grants, once per cluster
nginx/lb.conf                  the routing rule above, locally
scripts/e2e.sh                 the end-to-end checks
ui/                            React + Vite; ui/src/api/generated is generated too
Dockerfile                     api image: static binary, distroless, no UI in it
```

Dependencies point one way: `transport -> service -> domain`, and
`repository -> domain`. Nothing in `service` or `domain` imports `transport`, and
these answer whether that is still true rather than trusting the reviewer:

```
go list -deps ./internal/service/... | grep -c 'internal/transport'
go list -f '{{range .Imports}}{{println .}}{{end}}' ./internal/domain/... | grep -c '\.'
```

Both print `0`. The second counts imports of the domain packages themselves: a
dotted import path is either this module or a third party, and the domain is
allowed neither. (`go list -deps` on its own is useless here — the standard
library has packages called `internal/abi` and `internal/cpu`.)

## API

JSON keys are the spec's, so they are camelCase: `id`, `title`, `content`,
`created`, `expires`, and `expiresInDays` on the way in.

| method | path | |
|---|---|---|
| GET | `/api/snippet` | every snippet, newest first |
| GET | `/api/snippet/{id}` | one snippet, `404` JSON if absent |
| POST | `/api/snippet` | `{title, content, expiresInDays}` -> `201` + `Location`, `422` with per-field errors |
| DELETE | `/api/snippet/{id}` | `204`, `404` if absent |
| GET | `/healthz` | liveness: never touches the database |
| GET | `/readyz` | readiness: pings the database, `503` when it cannot |

Every `/api/snippet` request requires a live `session` cookie; `POST` and
`DELETE` also require an `Origin` in `TRUSTED_ORIGINS`. Every snippet belongs to
the session's user, every query is scoped to that owner, and another user's id
is `404` rather than `403`. Sessions live in
Postgres for seven days absolute, with no sliding renewal; the cookie is
`HttpOnly; Secure; SameSite=Lax`, and the database stores only its SHA-256 hash.
Expired rows are ignored by the lookup and removed at boot. The page shows a
signed-out state until the sign-in UI lands (#56).

Sign-in is the server-side Google authorization-code flow: `/api/auth/google`
redirects, `/api/auth/google/callback` exchanges the code and verifies the ID
token. PKCE, state and nonce travel in short-lived HttpOnly cookies and are
single-use. There is no Google JavaScript on the page. `/api/auth/google` and
the callback are browser navigation and deliberately outside `openapi.yaml`;
`GET /api/auth/session` and `POST /api/auth/signout` are the UI's own control
endpoints, also hand-written. The page uses `/api/auth/session` to decide
between the signed-out prompt and the signed-in header, and sign-out deletes
the database row before clearing the cookie.

Any verified Google account may sign in; there is no allowlist. That is a
deliberate platform default, not an inherited demo accident. A deployment that
needs a restriction enforces it on the verified identity before creating the
user row.

## Tests

```
make db-bootstrap   # the app role and its grants (Docker does it on an empty volume)
make db-test        # create, bootstrap and migrate snippet_test
make codegen-check  # host: fails if the generated code is stale
make lint           # host: gofmt, staticcheck and oxlint; warnings fail
make tls-check      # host: CA and server certificate expiry; fails inside 30 days
make test-unit      # host, no database, no .env (-short)
make test           # host against the test database; fails without TEST_DATABASE_URL
make test-race      # host: all tests with the race detector
npm --prefix ui test # Vitest component tests
make e2e            # against the running stack on :8080
make vuln           # govulncheck: vulnerabilities reachable from this code
make vuln-images    # trivy: fixable HIGH/CRITICAL in both images
```

`make db-test` starts postgres if needed, creates `snippet_test`, applies the
application-role bootstrap and the migrations. It is safe to re-run. `make test`
clears `snippets` with the `psql` client before the run, so a failed run cannot
poison the next one, and it never writes to the development database.

`make lint` runs `gofmt -l` (failing on output, not fixing it), a pinned
`staticcheck`, and `oxlint --deny-warnings`. `make test-race` runs the same
database tests under the race detector. The UI tests run under Vitest with
`jsdom` and Testing Library.

`make vuln` runs a pinned `govulncheck` via `go run`, so there is nothing to
install and `go.mod` is untouched; `make vuln-binary` runs the same scan against
`bin/godough`, which is what the image ships. `make vuln-images` runs a pinned
Trivy through `docker run` over both images, which is the only one of these that
sees the OS packages in a base layer. All exit non-zero on a finding.

CI runs `make vuln` and the two image scans on every pull request, and weekly on
a schedule, because the vulnerability databases change without the repository
doing so. Dependabot opens grouped weekly PRs for `gomod`, `npm`, `docker` and
`github-actions`; nothing auto-merges, so every bump runs the four required
checks like any other change. The npm one matters because the built bundle has no
lockfile inside the image, which is the one surface Trivy cannot see. The image gate blocks on *fixable* HIGH/CRITICAL only: a CVE with no
fixed version cannot be resolved by anyone, so it is reported, not gated.

`make test` is deliberately strict: with no database it fails rather than
skipping, so a green run means the queries ran. `make e2e` checks what a browser
sees — LB routing, SPA fallback, cache headers, JSON shapes, the rendered list,
and a create -> read -> delete round-trip.

CI runs both (`test`, `e2e`) and `master` requires both to merge:

```
go test -count=1 ./...     # -count=1: the database is state the test cache cannot see
```

## Dependencies

`go.sum` describes the module graph, which is not the same set of modules the
binary contains: 20 lines there, 16 modules in the graph, 6 linked into the
binary. These answer the question that matters:

```
make deps           # modules linked into bin/godough, from its buildinfo
make deps-image     # the same, read from the binary inside the api image
make deps-build     # what the build will link              (go list -deps)
make deps-graph     # the whole graph, test-only included   (go list -m all)
make why            # why a module is here at all: make why MOD=<module>
make symbols        # function symbols per module in the binary, largest first
```

`make symbols` needs an unstripped binary. The image is built with
`-ldflags="-s -w"`, so `go tool nm` finds no symbol table there — which is why
`make vuln-binary` exists: it reads the pclntab instead.

## Conventions

One branch per issue, cut from `master`, named for the issue (`feat/13-...`).
One PR per branch; `test` and `e2e` must pass; the branch is deleted on merge.
No agent merges, and nobody pushes to `master` directly.

Pending work lives in the issue tracker, not here — a list of what is missing
becomes wrong the moment it is fixed.

## Publishing images

```
make ecr-login ECR_REGISTRY=<registry>
make ecr-push ECR_REGISTRY=<registry> TAG=<commit-sha>
make ecr-pull-test ECR_REGISTRY=<registry> TAG=<commit-sha>
```

`ecr-pull-test` pulls the pushed images back, starts the compose stack from them and runs `make e2e`.

The CI `publish` job runs `make ecr-push` on `master` after `test` and `e2e`, authenticating through GitHub OIDC. The ECR stack is in `infra/terraform/ecr`.

## License

MIT — see [LICENSE](LICENSE).
