# Prioritised backlog

Reviewed against `origin/master` at `55d64ff`.

**The snippet app is the fixture; the platform is the product.** Priority goes
to work every future CRUD SaaS inherits: identity, tenancy, hermetic tests,
observability, deployment and only then edge polish. Snippet behaviour that is
not a reusable pattern does not earn a place in the queue.

## P0 — platform foundation — done

| Issue | Result |
|---|---|
| #45 | Dedicated `snippet_test` database; `make test` cannot touch the development database |
| #47 | `make lint`, `make test-race`, Vitest UI tests, CI gates |
| #44 | Structured logs, `X-Request-ID`, one access line per request |

## P1 — SaaS identity and tenancy

| Order | Issue | Why | Depends on |
|---|---|---|---|
| 1 | #52 + #55 — sessions and CSRF | The first cookie-authenticated write is the exposure | — |
| 2 | #53 — Google code flow and offline test seam | The provider flow every app copies | #52 |
| 3 | #54 — ownership, expand then contract | The tenancy pattern; needs an identity to scope to | #45, #52 |
| 4 | #56 — sign-in UI | Last, because a wrong API shape makes it expensive | #53, #55, #47 |
| 5 | #41 — rate limit the write endpoints | The useful key is the session | #52 |

Until #56 lands, the page renders a signed-out state; the API is the source of
truth for identity.

## P2 — deployment and topology

| Order | Issue | Why | Depends on |
|---|---|---|---|
| 6 | #38 — TLS to Postgres | The default DSN story for every deployed app | — |
| 7 | #5 — Kubernetes manifests and Ingress | Decides the production edge: ALB/Ingress, static assets, CDN, header and compression ownership | #38 |

## P3 — edge polish, after the topology decision

| Issue | Why deferred |
|---|---|
| #48 | Image size and the nginx-vs-distroless decision are moot if static assets move to S3/CDN or the ALB/Ingress owns the edge |
| #42 | Headers and CSP may belong to the edge, not the app |
| #43 | Compression may belong to the CDN/edge, not the app |

Do not implement #48/#42/#43 until #5 records which layer owns each behaviour.

## Deferred — snippet-domain work

- **#12** — expiry filtering. Snippet-domain logic.
- **#35** — pagination. The reusable list pattern matters, but not before
  identity and tenancy.
- **#40** — content cap. The reusable part is request validation; revisit when a
  real app needs that pattern.

## Done

- **#34** — probes stay on the API pod.
- **#44** — structured logs and request ids.
- **#45** — hermetic test database.
- **#46** — TypeScript generation delivered by #58.
- **#47** — lint, race and UI gates.
- **#59** — spec-first API epic.

## Do not start yet

- #54 before #45 and #52.
- #56 before #53 and #47.
- #48/#42/#43 before #5's topology decision.
- #5 before #38.
- #12, #35 or #40 before identity and tenancy.

## Working rules

- One ticket, one branch, one PR. Do not start the next one until the current
  one is merged and closed by the human.
- The author does not verify; the verifier does not merge.
- Acceptance evidence belongs in a comment with the SHA it ran against.
