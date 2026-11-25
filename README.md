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

## API

| method | path | |
|---|---|---|
| GET | `/api/snippets` | every unexpired snippet, newest first |
| GET | `/api/snippet/view/{id}` | one snippet, `404` JSON if absent |
| POST | `/api/snippet/create` | `{Title, Content, ExpiresInDays}` -> `201` + `Location`, `422` with per-field errors |
| DELETE | `/api/snippet/view/{id}` | `204`, `404` if absent |
| GET | `/healthz` | liveness: never touches the database |
| GET | `/readyz` | readiness: pings the database, `503` when it cannot |

## Tests

```
make test-unit      # host, no database, no .env (-short)
make test           # host against the postgres container; fails without it
make e2e            # against the running stack on :8080
```

`make test` is deliberately strict: with no database it fails rather than
skipping, so a green run means the queries ran. `make e2e` checks what a browser
sees — LB routing, SPA fallback, cache headers, JSON shapes, the rendered list,
and a create -> read -> delete round-trip.

CI runs both (`test`, `e2e`) and `master` requires both to merge:

```
go test -count=1 ./...     # -count=1: the database is state the test cache cannot see
```

## Layout

```
cmd/web/                     the API: routes, handlers, db, health
migrations/                  plain SQL, applied by make db-migrate
docker-entrypoint-initdb.d/  roles and grants, run once on an empty volume
nginx/lb.conf                the routing rule above, locally
scripts/e2e.sh               the end-to-end checks
ui/                          React + Vite; ui/nginx.conf serves the built SPA
Dockerfile                   api image: static binary, distroless, no UI in it
```

## Conventions

One branch per issue, cut from `master`, named for the issue (`feat/13-...`).
One PR per branch; `test` and `e2e` must pass; the branch is deleted on merge.
No agent merges, and nobody pushes to `master` directly.

Pending work lives in the issue tracker, not here — a list of what is missing
becomes wrong the moment it is fixed.

## License

MIT — see [LICENSE](LICENSE).
