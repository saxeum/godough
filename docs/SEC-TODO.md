# Security todo

Work list from [`security-review.md`](security-review.md), reviewed against the
live site and `master` at `655d47e` on 2026-11-27. The review holds the evidence;
this file holds the decisions, so an item that changes is corrected here rather
than left as a stale claim.

Order is by cost of not doing it, not by severity alone: the small mechanical
fixes come first because they are one-line changes that close a finding
completely. Nothing here is a ticket yet — file one per item (or per tight group)
before starting, cut from `master`, named for the change.

Rules are the project's: one branch, one pull request, the author does not
verify, the verifier does not merge, and no agent pushes to `master`.

## P1 — do first, each is small and independently shippable

- [ ] **E1 — ALB TLS policy.** Add
  `alb.ingress.kubernetes.io/ssl-policy: ELBSecurityPolicy-TLS13-1-2-2021-06`
  to both Ingress templates (`deploy/charts/api/templates/ingress.yaml` and
  `deploy/charts/ui/templates/ingress.yaml`). Close when `ssl-enum-ciphers`
  reports no TLSv1.0 or TLSv1.1 suites and TLSv1.3 is offered.

- [ ] **E2/E3 — response headers.** Add `Strict-Transport-Security`,
  `Content-Security-Policy` (with `frame-ancestors 'none'`), `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: strict-origin-when-cross-origin` and a minimal
  `Permissions-Policy` in `ui/nginx.conf`; add the non-CSP subset in
  `writeJSON` so API responses carry them too. Close when the header set from
  `curl -sSI https://godough.foonix.net/` contains all five and the SPA still
  loads with no console CSP violation, and a create/edit/delete round trip
  through the browser still works.

- [ ] **A2 — `Cache-Control: no-store`.** One `Header().Set` in
  `internal/transport/httpapi/respond.go`. Close when an authenticated
  `GET /api/snippet` response carries it.

- [ ] **G2 — require `iac`.** Add `iac` to the required status checks on
  `master`. Close when `gh api repos/saxeum/godough/branches/master/protection`
  lists it.

- [ ] **G3 — `strict: true`.** Turn on "require branches to be up to date"
  so the reviewed tree is the merged tree. Close when the same call reports
  `"strict": true`.

- [ ] **G1 — a pull-request rule on `master`.** Add a ruleset rule of type
  `pull_request` with one required approval (and dismiss stale approvals on
  push), keeping the existing `deletion` and `non_fast_forward` rules. This is
  the control behind "no agent merges"; today only the convention exists.
  Close when a direct push to `master` is rejected and a PR without an approval
  cannot merge.

- [ ] **D1 — deletion protection.** `deletion_protection = true` on
  `aws_db_instance.postgres` and on the ALB (`alb.ingress.kubernetes.io/load-balancer-attributes:
  deletion_protection.enabled=true`). Close when `aws rds describe-db-instances`
  reports it and the ALB attribute reads `true`.

- [ ] **E5/K5 — see the traffic, then shape it.** In this order: ALB access logs
  to a bucket with a lifecycle rule, then WAF (AWS managed common rule set plus
  a rate-based rule on `/api/*`), then GuardDuty. Logs first, because without
  them a WAF rule cannot be tuned and an incident cannot be reconstructed.
  Close when an attacker-shaped burst appears in the logs and is blocked by a
  named rule.

## P2 — the app's own limits, after the edge is observable

- [ ] **A4, then A1 — client address, then rate limiting.** Parse
  `X-Forwarded-For` from the right, trusting only the ALB hop, and use it for
  `sessions.ip`; then a token bucket keyed on the authenticated user and falling
  back to that address. Raise `pool_max_conns` in step: five connections is the
  reason a small burst is fatal. Close when a sustained unauthorised flood is
  throttled with a 429 and an authenticated user's rows show a real address.

- [ ] **A1 — per-user quota.** Cap snippets per user and cap `content` in the
  service, not only in the transport. Close when the 1 MiB guard is no longer the
  only bound.

- [ ] **D2 — stop keeping the owner credential.** Create `godough-owner` in the
  pre-upgrade hook and delete it after, or move to the Secrets Manager CSI driver
  with IRSA (`make k8s-secrets` currently writes it by hand). Enable rotation on
  `godough-staging/app-password` either way. Close when `kubectl get secrets -n
  godough` has no long-lived owner DSN outside a migration window.

- [ ] **K1 — default-deny NetworkPolicy.** api may reach RDS on 5432 and DNS;
  ui may reach nothing; both may be reached from the ALB. Close when a pod
  launched in `default` cannot open 5432 to RDS.

- [ ] **K2 — IMDS hop limit 1.** Drop `http_put_response_hop_limit` from 2 to 1
  in `infra/terraform/eks/main.tf`. The VPC CNI runs `hostNetwork: true` and does
  not need the second hop; verify it still starts before closing.

## P3 — hardening, batch these

- [ ] **A3** — rename the session cookie to `__Host-session` (host-only, `Path=/`,
  `Secure` are then enforced by the browser). The zone is shared, so a sibling
  subdomain can otherwise set a `Domain=.foonix.net` cookie of the same name.
- [ ] **K3** — `automountServiceAccountToken: false` on both Deployments; add
  IRSA only when something actually needs AWS.
- [ ] **K4** — label namespace `godough` with `pod-security.kubernetes.io/enforce: restricted`.
- [ ] **D3** — enable account EBS default encryption, and give the node launch
  template an encrypted `block_device_mappings` entry. Existing volumes stay
  unencrypted until replaced; say so rather than implying a backfill.
- [ ] **K7** — `aws ecr start-image-scan` on the two deployed digests, and make
  sure the scan-on-push result is what CI reads rather than a record nobody looks
  at.
- [ ] **E4/E6** — `server_tokens off` in `ui/nginx.conf`;
  `alb.ingress.kubernetes.io/load-balancer-attributes: routing.http.drop_invalid_header_fields.enabled=true`.
- [ ] **E9** — route the mux's own 404 through `writeJSON` so every API error is
  JSON.
- [ ] **K6** — apply the pending `data` stack tag changes before an ALB is ever
  recreated.
- [ ] **G4** — decide on `sha_pinning_required`: the workflows already pin by
  SHA, so setting it makes the next omission a failure rather than a review catch.
- [ ] **A6** — if an email allowlist is ever added, check `email_verified` first.
  Today `sub` is the key, so this is a note for whoever writes the allowlist.

## Decisions this file does not make

- **Open registration.** Any verified Google account may sign in; the README calls
  that a deliberate platform default. With A1/A5 done, that is defensible. Without
  them it is a public, unmetered write endpoint. Decide which comes first.
- **Where the edge policy lives.** E1/E2/E3/E5 can be enforced by the ALB
  (annotations, WAF, response headers on the listener) or by the app
  (middleware). Doing it in both places is volume; pick one and say which in the
  ticket.
- **Staging vs production posture.** D1, D2 and multi-AZ are cheap to defer on
  staging and expensive to discover in production. If this account is ever
  promoted, this list is the promotion checklist, not a suggestion.

## Re-check sweep

Run these before closing the epics above, and again before any promotion. Each is
read-only.

```
nmap -Pn -p443 --script ssl-enum-ciphers godough.foonix.net
```

Expect no TLSv1.0/1.1 blocks and a TLSv1.3 block, once E1 is closed.

```
curl -sSI https://godough.foonix.net/
```

Expect `strict-transport-security` and `content-security-policy` in the headers,
once E2/E3 is closed.

```
gh api repos/saxeum/godough/rules/branches/master
```

Expect `pull_request` alongside `deletion` and `non_fast_forward`, once G1 is
closed.

```
kubectl get netpol -A
```

Expect at least one policy in `godough`, once K1 is closed.

## Closed

Move items here with the SHA that closed them and the command output that shows
it. A claim and its evidence live in different places: the claim is edited here,
the evidence is a comment on the ticket.
