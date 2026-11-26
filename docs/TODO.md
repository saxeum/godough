# Prioritised backlog

Reviewed against `origin/master` at `88807b1`. The issue bodies were refreshed
in the same pass: stale claims and acceptance commands now match the tree.

Priority is about **what is exposed or what unblocks other work**, not about
what is easiest. The order inside each tier is the recommended execution order.

## P0 — make the request path safe and the probes honest

| Order | Issue | Why now | Depends on |
|---|---|---|---|
| 1 | #34 — keep `/healthz` and `/readyz` on the API pod; stop the SPA fallback answering them | The ALB target group and kubelet check the pod directly; the web tier must not answer a probe path with `200` | — |
| 2 | #40 — cap `content` and align the 413 with nginx | Sets the per-row bound; the transport `413` JSON half already exists | — |
| 3 | #12 — filter expired snippets out of the read paths | Small correctness fix; edits the same list query #35 will rewrite | — |
| 4 | #35 — paginate `GET /api/snippet` | Bounds the list. Worst case is `page size × content cap` | #40, #12 |

Do #12 before #35 so the list query is touched once. Do #40 before #35 so the
page-size decision has a real per-row ceiling to multiply by.

## P1 — make the suite able to catch the rest

| Order | Issue | Why now | Depends on |
|---|---|---|---|
| 5 | #45 — dedicated test database | Unblocks #54; removes the last e2e seed/id dependency | — |
| 6 | #47 — `gofmt`, `staticcheck`, `-race`, `oxlint`, UI tests | `-race` before the session work; UI tests before #56 | — |
| 7 | #44 — JSON logs, request ids, access log | Needed to see #35/#41 abuse and to correlate failures | — |
| 8 | #48 — drop dead static files; decide the web runtime | The runtime decision determines whether #42/#43 live in nginx or Go | — |
| 9 | #42 — security headers and a CSP | No third-party font any more (#89), so a strict policy is available | #48 |
| 10 | #43 — gzip on both surfaces | Same two nginx files as #42; do them together | #48 |

#48's decision comes before #42/#43, not after: choosing `distroless/static`
deletes `ui/nginx.conf` and moves both changes into Go middleware.

## P2 — the auth epic

| Order | Issue | Why now | Depends on |
|---|---|---|---|
| 11 | #52 + #55 — sessions and CSRF | Land together. The first cookie-authenticated write is the exposure | — |
| 12 | #53 — Google code flow and offline test seam | Needs the session store; the seam is what makes it testable | #52 |
| 13 | #54 — ownership, expand then contract | Two deploys; needs an identity to scope to | #45, #52 |
| 14 | #56 — sign-in UI | Last, because a wrong API shape makes it expensive | #53, #55, #47 |
| 15 | #41 — rate limit the write endpoints | The useful key is the session, not the address | #52 |

If the API can be publicly reachable before P2 lands, pull the address-keyed
version of #41 forward. Otherwise it is cheaper after auth.

## P3 — deploy

| Order | Issue | Why now | Depends on |
|---|---|---|---|
| 16 | #38 — TLS to Postgres | Negative case runs locally; positive case needs RDS/staging | — |
| 17 | #5 — Kubernetes manifests and Ingress | Last; the packaging depends on the rest | #34, #38, #48 |

## Not work items

- **#59** — spec-first API is delivered by #58. A close comment with evidence is
  on the ticket; a human should close it.
- **#46** — TypeScript generation is delivered by #58. Only the `updated_at` /
  `DELETE` decision remains; record it in the spec and close.
- **#49, #50, #51, #57** — trackers. Their bodies now show which children are
  delivered and which remain.

## Do not start yet

- #5 before #34, #38 and #48.
- #54 before #45 and #52.
- #56 before #53 and #47.
- #42/#43 before #48's runtime decision.
- #41 before deciding whether the API can be publicly reachable pre-auth.

## Working rules

- One ticket, one branch, one PR. Do not start the next one until the current
  one is merged and closed by the human.
- The author does not verify; the verifier does not merge.
- Acceptance evidence belongs in a comment with the SHA it ran against.
