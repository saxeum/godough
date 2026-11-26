# Prioritised backlog

Reviewed against `origin/master` at `06a2737`.

**The snippet app is the fixture; the platform is the product.** Priority goes
to work every future CRUD SaaS inherits: hermetic tests, gates, observability,
edge defaults, identity, tenancy and deployment. Snippet behaviour that is not a
reusable pattern does not earn a place in the queue.

## P0 — platform foundation

| Order | Issue | Why | Depends on |
|---|---|---|---|
| 1 | #45 — dedicated test database | Every future app needs a hermetic database, and #54 needs it | — |
| 2 | #47 — `gofmt`, `staticcheck`, `-race`, `oxlint`, UI tests | Gates belong in the template, not added per app | — |
| 3 | #44 — JSON logs, request ids, access log | Observability is platform, not snippet | — |

## P1 — edge and packaging defaults

| Order | Issue | Why | Depends on |
|---|---|---|---|
| 4 | #48 — drop dead static files; decide the web runtime | The runtime decision determines where the edge behaviour lives | — |
| 5 | #42 — security headers and a CSP | Reusable edge defaults; no third-party font any more (#89) | #48 |
| 6 | #43 — gzip on both surfaces | Same two nginx files as #42; do them together | #48 |
| 7 | #38 — TLS to Postgres | The default DSN story for every deployed app | — |

## P2 — SaaS identity and tenancy

| Order | Issue | Why | Depends on |
|---|---|---|---|
| 8 | #52 + #55 — sessions and CSRF | The first cookie-authenticated write is the exposure | — |
| 9 | #53 — Google code flow and offline test seam | The provider flow every app copies | #52 |
| 10 | #54 — ownership, expand then contract | The tenancy pattern; needs an identity to scope to | #45, #52 |
| 11 | #56 — sign-in UI | Last, because a wrong API shape makes it expensive | #53, #55, #47 |
| 12 | #41 — rate limit the write endpoints | The useful key is the session | #52 |

## P3 — delivery

| Order | Issue | Why | Depends on |
|---|---|---|---|
| 13 | #5 — Kubernetes manifests and Ingress | Packaging and deploy, after the platform pieces | #38, #48 |

## Deferred — snippet-domain work

These are not platform work. Leave them open unless the fixture needs them.

- **#12** — expiry filtering. Snippet-domain logic.
- **#35** — pagination. The reusable list pattern matters, but not before
  identity and observability, and it loses its bound while #40 is deferred.
- **#40** — content cap. The reusable part is request validation; revisit when a
  real app needs that pattern.

## Done

- **#34** — probes stay on the API pod; the SPA no longer answers them. PR #92
  merged; the ticket still needs a human to close it.
- **#46** — TypeScript generation delivered by #58. Closed.
- **#59** — spec-first API epic. Closed.

## Do not start yet

- #54 before #45 and #52.
- #56 before #53 and #47.
- #42/#43 before #48's runtime decision.
- #5 before #38 and #48.
- #12, #35 or #40 before the platform foundation.

## Working rules

- One ticket, one branch, one PR. Do not start the next one until the current
  one is merged and closed by the human.
- The author does not verify; the verifier does not merge.
- Acceptance evidence belongs in a comment with the SHA it ran against.
