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

Then <http://localhost:8080> lists three seed snippets. `make down` stops the
containers and keeps the database volume; `docker compose down -v` wipes it.

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

`.env` is read by host targets only. Compose sets `DATABASE_URL` itself, because
the database is `postgres:5432` inside the network and `127.0.0.1:5432` outside.

- `SNIPPET_ADDR` — host:port the API listens on (`:4000`)
- `DATABASE_URL` — a pgx DSN; the app connects as `godough_app`, a non-superuser
  that owns nothing, created by `docker-entrypoint-initdb.d/01-app-role.sql`
- Migrations run as `godough`, the owner, over the container's trusted socket

Both variables are required. There are no defaults: a missing one exits at
startup with its name.

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
docker-entrypoint-initdb.d/    roles and grants, run once on an empty volume
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
go list -deps ./internal/domain/...  | grep -cE 'internal/|github\.com'
```

Both print `0`.

## API

JSON keys are the spec's, so they are camelCase: `id`, `title`, `content`,
`created`, `expires`, and `expiresInDays` on the way in.

| method | path | |
|---|---|---|
| GET | `/api/snippet` | every snippet, newest first |
| GET | `/api/snippet/{id}` | one snippet, `404` JSON if absent |
| POST | `/api/snippet` | `{Title, Content, ExpiresInDays}` -> `201` + `Location`, `422` with per-field errors |
| DELETE | `/api/snippet/{id}` | `204`, `404` if absent |
| GET | `/healthz` | liveness: never touches the database |
| GET | `/readyz` | readiness: pings the database, `503` when it cannot |

## Tests

```
make codegen-check  # host: fails if the generated code is stale
make test-unit      # host, no database, no .env (-short)
make test           # host against the postgres container; fails without it
make e2e            # against the running stack on :8080
make vuln           # govulncheck: vulnerabilities reachable from this code
make vuln-images    # trivy: fixable HIGH/CRITICAL in both images
```

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

## License

MIT — see [LICENSE](LICENSE).
