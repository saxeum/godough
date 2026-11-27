# Security review — godough

**Date:** 2026-11-27
**Live:** <https://godough.foonix.net> (ALB → EKS, namespace `godough`)
**Tree reviewed:** `ab84f39` on `ops/116-tlscheck-in-image`; `origin/master` = `655d47e`
**Live images:** `godough-api@sha256:7a89b192…`, `godough-web@sha256:09984196…`, ECR tag `d24bfac`

`git diff d24bfac..origin/master -- cmd internal ui spec migrations db` is empty, so
the application code running in the cluster is exactly what `master` contains. The
deployed image predates `master` only in files that are not in that path.

This is a point-in-time record. The findings below were true when the commands
were run; a description that is corrected later is corrected here, not in a
comment on it. Everything actionable is in [`SEC-TODO.md`](SEC-TODO.md), which is
the file to keep current.

## Scope and method

| Surface | What was done |
|---|---|
| App code | Full read of `cmd/`, `internal/`, `migrations/`, `db/`, `spec/`, `ui/src/` |
| Local deploy | compose stack inspected; authenticated behaviour exercised with a fixture session (CSRF, ownership, validation, limits) |
| AWS | Read-only: ELBv2, RDS, ECR, S3, IAM, EBS, CloudTrail, `terraform state`, `plan -refresh` |
| Kubernetes | Read-only through the existing kubeconfig: workloads, RBAC, Secrets (names and keys only), launch template |
| GitHub | Repo settings, branch protection, rulesets, alert counts, workflow definitions |
| Live site | Unauthenticated probing only: routing, headers, TLS, OAuth start and callback, cookie shape, path handling, method matrix |
| Scanners | `make vuln` (govulncheck), `npm audit`, Trivy on the deployed digests |

**Not done, deliberately.** No volumetric or destructive testing, no credential
attack, no completed Google sign-in. Authenticated tests therefore ran against
the local stack, which is built from the same source commit but is a different
image digest. The origin check and the ownership scoping are verified from code
and from a local run, not on the live host. Treat those two as "reviewed", not
"proven live".

## Findings

| # | Sev | Finding |
|---|---|---|
| E1 | Med | ALB accepts TLS 1.0/1.1, offers no TLS 1.3, allows static-RSA key exchange and CBC suites |
| E2 | Med | No HSTS |
| E3 | Med | No CSP, `X-Frame-Options`, `nosniff` or `Referrer-Policy`; clickjacking reaches delete and sign-out |
| E5 | Med | No WAF, no ALB access logs, no rate limiting: cheap DoS of a 1-replica, 5-connection app |
| A1 | Med | No anti-automation on any endpoint; any Google account can create unbounded data |
| G1 | Med | `master` requires no pull request and no review; only `deletion` and `non_fast_forward` |
| G2 | Med | `iac` (the Terraform policy scan) is not a required check |
| D1 | Med | RDS `deletion_protection = false`, `skip_final_snapshot = true` |
| D2 | Med | The RDS **master** credential sits in cluster Secret `godough-owner` indefinitely |
| K1 | Med | No NetworkPolicy: any pod can reach RDS:5432 and every other pod |
| K2 | Med | IMDS hop limit 2, so a pod can take the node instance-profile credentials |
| K5 | Med | No GuardDuty, Config or Security Hub; CloudTrail log-file validation off |
| A2 | Low | Authenticated JSON has no `Cache-Control: no-store` |
| A3 | Low | Session cookie is `session`, not `__Host-session`, on a zone shared with other subdomains |
| A4 | Low | Session IP is the ALB's, not the client's; `X-Forwarded-For` is ignored |
| A5 | Low | No per-user quota; content is bounded only by the 1 MiB request guard |
| D3 | Low | Node EBS volumes are unencrypted: account EBS default encryption is off |
| K3 | Low | The default ServiceAccount token is automounted in the app pod; no IRSA |
| K4 | Low | The namespace has no Pod Security Admission labels |
| K7 | Low | ECR `scanOnPush` has never produced findings for the digests that are deployed |
| G3 | Low | `strict: false`: a pull request may merge while behind `master` |
| G4 | Low | `allowed_actions: all`, `sha_pinning_required: false`; the pinning is by hand |
| E4 | Info | `Server: nginx/1.31.7` and `awselb/2.0` version disclosure |
| E6 | Info | ALB `drop_invalid_header_fields = false` |
| E9 | Info | `/api/nope` answers `text/plain 404` where every other API error is JSON |
| K6 | Info | `data` stack drift: two subnet tag updates pending (the ALB discovery tags) |

## Edge and live HTTP

### E1 — The TLS policy is the 2016 default (Med)

Observed listener configuration:

```
Port  Protocol  SslPolicy
80    HTTP      None
443   HTTPS     ELBSecurityPolicy-2016-08
```

`nmap -Pn -p443 --script ssl-enum-ciphers godough.foonix.net` enumerates TLSv1.0
and TLSv1.1 as accepted, each with twelve suites including
`TLS_RSA_WITH_AES_128_CBC_SHA` (static RSA key exchange) and CBC suites, and
TLSv1.2 with the same set. No TLS 1.3 is offered: `curl --tlsv1.3` fails while
`--tlsv1.2` succeeds.

The fix is one annotation on both Ingresses:
`alb.ingress.kubernetes.io/ssl-policy: ELBSecurityPolicy-TLS13-1-2-2021-06`.

### E2/E3 — Response headers (Med)

The complete header set from the live origin:

```
HTTP/2 200
server: nginx/1.31.7
content-type: text/html
content-length: 466
last-modified: Fri, 27 Nov 2026 00:05:28 GMT
etag: "6b08c948-1d2"
cache-control: no-store
accept-ranges: bytes
```

Missing: `Strict-Transport-Security`, `Content-Security-Policy`,
`X-Frame-Options`/`frame-ancestors`, `X-Content-Type-Options`,
`Referrer-Policy`, `Permissions-Policy`. The API's 200 and 401 responses are
equally bare.

The frame-ancestors gap is not cosmetic. `requireTrustedOrigin` compares the
`Origin` header, and a request issued from inside a framed copy of the app
carries the app's own origin, so it passes the check. Locally:

```
curl -X POST -H 'Origin: http://evil.example' -b 'session=…' /api/snippet   -> 403 {"error":"Forbidden"}
curl -X POST                     (no Origin)  -b 'session=…' /api/snippet   -> 403
curl -X POST -H 'Origin: http://localhost:8080' -b 'session=…' /api/snippet -> 201
```

So a framed page can drive a user's own authenticated delete or sign-out.
`index.html` is `no-store`, the bundle is self-hosted, and there is no inline
script or third-party origin, so a strict `default-src 'self'` policy should
apply without loosening the build.

### E5/A1 — No rate limiting, no WAF, no request logs (Med)

```
aws wafv2 list-web-acls --scope REGIONAL          -> (empty)

access_logs.s3.enabled                             false
health_check_logs.s3.enabled                       false
connection_logs.s3.enabled                         false
routing.http.desync_mitigation_mode                defensive
```

The live DSN sets `pool_max_conns=5` and the chart runs `replicaCount: 1`. Every
unauthenticated request to `/api/snippet` costs a database round trip, because the
session lookup happens before the request is rejected; a garbage cookie returns
401 only after that query. There is no limiter in the handler chain
(`withRequestID → accessLog → recoverer → requestDeadline → mux`). A modest flood
therefore saturates five connections, and no component records who sent it.
`TODO.md` defers this as #41, "rate limiting needs the deployed edge and the
session key" — the edge now exists, and so does the session key.

The same absence means any verified Google account can create snippets without
limit (`content` is `text`, bounded only by the 1 MiB body guard) and any
anonymous client can hammer `/api/auth/google`, which mints three cookies per
request.

### E9 — Routing and consistency details (Info)

```
/                      200 text/html      /apifoo    200 text/html   (the /api prefix does not match)
/api                   404 text/plain     /api/nope  404 text/plain
/api/snippet           401 application/json
/api/../  /api/..%2f   200 text/html      (normalised to / by the ALB; not a bypass)
/%2e%2e/api/snippet    400 text/html      (rejected by the ALB)
```

Host handling is correct: `Host: evil.example` against the real SNI returns the
ALB's fixed `404`, there is no host confusion, and there is no open redirect —
`X-Forwarded-Host` is ignored and the redirect location is Google's fixed URL
carrying the configured `redirect_uri`.

## Application

### What held up

| Test | Result |
|---|---|
| Another owner's snippet id | `404` for both GET and DELETE, not `403` |
| Write with no Origin, a foreign Origin, `null`, or a trailing slash | `403` |
| Write with the trusted Origin | `201` |
| Session cookie absent, garbage, or duplicated | `401` |
| `/api/snippet/9223372036854775808` | `400`, no 500 |
| 101-rune title; `expiresInDays` 0 and 366 | `422` with per-field messages |
| 1.1 MiB body, direct to the API | `413` JSON; through the LB nginx answers first |
| `text/plain` body | `401`: the session check runs first, by design |
| `TRACE` | `405`; `OPTIONS` on a protected path `401` |
| `X-Request-ID: <script>…` and a 200-character id | replaced with a generated id (safe-character allowlist) |
| `/.env`, `/.git/config`, `/robots.txt` | SPA fallback; no file leak |

The cookie shape, from the only place cookies are set:

```
set-cookie: oauth_state=…; Path=/api/auth; Max-Age=600; HttpOnly; Secure; SameSite=Lax
```

PKCE (`S256`), `nonce` and `state` are all present. The callback consumes and
clears all three before any check, compares `state` in constant time, and creates
a new session token, so there is no fixation. A callback with no flow cookies
returns `400` and sets no session.

### A2 — No `Cache-Control` on authenticated JSON (Low)

```
HTTP/1.1 200 OK
Content-Type: application/json
X-Request-Id: …
```

`/api/auth/session` returns the user's email and name; `/api/snippet` returns
content. Nothing in the path caches today, but this is one CDN or proxy away from
mattering. `writeJSON` is the single writer for non-generated bodies, so one
header set there covers every JSON response.

### A3/A4 — Cookie naming and client IP (Low)

`session` is a generic, host-only cookie on `godough.foonix.net`, a name in the
`foonix.net` zone held in Cloudflare. Any sibling subdomain can set a
`Domain=.foonix.net` cookie of the same name; `__Host-session` makes a spoofed
cookie inert by construction.

Separately, `clientIP(r)` reads `r.RemoteAddr`, which behind the ALB is the load
balancer's node address. Every `sessions.ip` row therefore records the ALB, so the
column cannot answer "where did this session come from", and an IP-keyed limiter
built on it would key on one address. The ALB sets `X-Forwarded-For`
(`xff_header_processing.mode: append`); it is simply unused.

### A5/A6 — Quotas and claims (Low/Info)

There is no per-user snippet cap and no service or column limit on `content`.
`googleauth.Exchange` verifies issuer, audience, nonce and `sub` but not
`email_verified`. That is safe today because `sub` is the identity key and email
is display-only, but it becomes a bug the day an allowlist is written against an
address — which the README already anticipates.

## Data

### D1 — Deletion protection (Med)

```
id godough-staging | public False | enc True | deletion_protection False
backup_retention_period 7 | skip_final_snapshot true
```

A mistaken `terraform destroy` or a console delete takes the database with no
final snapshot; recovery is the seven-day point-in-time window.
`deletion_protection = true` costs nothing and is the cheap half of the guarantee.

### D2 — The master credential outlives the migration (Med)

```
kubectl get secrets -n godough
godough-api      1   (the application role's DSN)
godough-owner    1   (the owner DSN: the RDS master password)
godough-oauth    3   (client id, client secret, redirect URL)
```

`godough-owner` exists so the pre-upgrade hook can migrate. The hook has
`hook-delete-policy: before-hook-creation,hook-succeeded`, so the Job is removed,
but the Secret stays for the life of the release. The namespace has no RBAC
restricting `get secrets`, and `godough-staging/app-password` reports
`RotationEnabled: None`. None of this is reachable from the internet, but a
resident cluster secret holding the superuser password is the largest single
credential in the system. Creating it in the hook and deleting it after, or
moving to the Secrets Manager CSI driver with IRSA, removes it.

### D3 — Unencrypted node volumes (Low)

```
aws ec2 get-ebs-encryption-by-default        {"EbsEncryptionByDefault": false}
vol-066b705cd545f1b9f  Encrypted False  20 GiB
vol-0fd5156682c70d771  Encrypted False  20 GiB
```

The bastion's root volume is explicitly encrypted. The two node volumes are not,
because the launch template sets no `block_device_mappings` and the account
default is off.

### What is right here

RDS is `publicly_accessible: false`, in private subnets, and reachable on 5432
only from the node security group and the bastion. Storage is encrypted,
`rds.force_ssl = 1`, and the app connects with
`sslmode=verify-full&sslrootcert=/certs/global-bundle.pem`. Sessions store only
the SHA-256 of a 256-bit token. The application role is `NOSUPERUSER NOCREATEDB
NOCREATEROLE` and owns nothing.

## AWS and Kubernetes

### K1/K2 — Blast radius inside the cluster (Med)

```
kubectl get netpol -A        No resources found

launch template MetadataOptions
{"hop": 2, "tokens": "required", "sgs": ["sg-005c087264b0e3474"]}
```

With no NetworkPolicy, anything that lands in the cluster reaches RDS:5432 (the
node security group is allowed there) and every other pod. Combined with hop
limit 2, a compromised pod can read the node instance profile from IMDS:
`AmazonEKSWorkerNodePolicy`, `AmazonEKS_CNI_Policy`,
`AmazonEC2ContainerRegistryReadOnly`. The VPC CNI that motivated the hop limit
runs `hostNetwork: true` and does not need it. IMDSv2 with hop limit 1 is the
EKS recommendation.

The workload itself is well built: `runAsNonRoot`,
`seccompProfile: RuntimeDefault`, `allowPrivilegeEscalation: false`,
`readOnlyRootFilesystem: true`, all capabilities dropped, distroless nonroot, and
digests rather than tags in the chart.

### K3/K4 — Namespace defaults (Low)

The app runs as `default` with a projected ServiceAccount token mounted. That
token has no permissions today (`auth can-i --list` shows only self-reviews), and
the namespace carries no `pod-security.kubernetes.io/enforce` label, so nothing
prevents a privileged pod being scheduled beside it.

### K5 — Account-level detection (Med)

```
cloudtrail: management-events, IsMultiRegionTrail True, LogFileValidationEnabled False
guardduty list-detectors                          -> (empty)
securityhub describe-hub                          -> not subscribed
config describe-configuration-recorders           -> (empty)
```

There is no detection layer: no behavioural findings, no configuration drift
record, no attack-path view, and a trail that cannot prove it was not edited.
This is the difference between finding out and finding out from the bill.

### K7 — The ECR scan gate is nominal (Low)

`aws ecr describe-image-scan-findings` for the deployed api digest returns
`ScanNotFoundException`, and `imageScanStatus` is null. `scan_on_push` is true,
but the digests that are actually running have no scan record. `make vuln-images`
in CI is the real gate and it passes, so this is a monitoring gap rather than an
unpatched risk; `aws ecr start-image-scan` on the two digests puts a record
behind them.

### K6 — State drift (Info)

```
terraform -chdir=infra/terraform/data plan …
  ~ aws_subnet.public[0] tags  + "kubernetes.io/role/elb" = "1"
  ~ aws_subnet.public[1] tags  + "kubernetes.io/role/elb" = "1"
Plan: 0 to add, 2 to change, 0 to destroy.
```

The ALB discovery tags are in the tree (`da0f1ff`) but not in the account. The
ALB exists and works; the next apply adds them. Worth knowing before an ALB is
ever recreated. The `eks` stack reports `No changes`.

### Exposed surface

```
nmap -Pn -p 22,80,443,4000,5432,8080,6443,10250 godough.foonix.net
80/tcp   open   http
443/tcp  open   https
everything else filtered

nmap -Pn -F <bastion public address>   all 100 ports filtered
```

Only the ALB is reachable. RDS, the API pod, kubelet and the EKS endpoint are not.

## GitHub and supply chain

### G1 — Nothing on `master` requires a pull request (Med)

```
gh api repos/saxeum/godough/rules/branches/master
['deletion', 'non_fast_forward']

gh api repos/saxeum/godough/branches/master/protection
required_status_checks: {strict: false, contexts: [test, e2e, vuln, images]}
enforce_admins: true, allow_force_pushes: false, allow_deletions: false
required_pull_request_reviews: (absent)
```

There is no `pull_request` rule and no approval requirement, so GitHub itself does
not require a commit to reach `master` through a pull request. The four required
checks gate merges, not pushes. That matters more than usual here because the
publish role trusts exactly that ref:

```
StringLike  token.actions.githubusercontent.com:sub
            repo:saxeum@343752441/godough@1472378230:ref:refs/heads/master
```

A push to `master` is a push to ECR with a token minted by OIDC: no static keys,
but also no reviewer in the path. The project's own invariants ("no agent merges",
"the reviewed object is the merged object") are currently enforced by convention.
Two settings make them mechanical: a `pull_request` ruleset rule with one required
approval, and `strict: true`.

### G2 — `iac` is not required (Med)

The required contexts are `test, e2e, vuln, images`. `security.yml` also defines
`iac`, the Trivy config scan over `infra/terraform`, and it is the only check that
would catch a public bucket or a wildcard IAM trust before apply. It runs on every
pull request and nobody has to wait for it.

### G3/G4 — Lower-severity settings (Low)

`strict: false` means a pull request reviewed at one tree can merge after
`master` moved. `allowed_actions: all` with `sha_pinning_required: false` leaves
the pin discipline to reviewers, who do pin by SHA with version comments. There is
no `CODEOWNERS`, so a change to `.github/workflows/` or `infra/terraform/` needs no
particular reviewer.

### Supply chain: what is strong

```
secret_scanning: enabled (+ push_protection, non_provider_patterns, validity_checks)
dependabot_security_updates: enabled
repo secrets: none        deploy keys: none        webhooks: none
dependabot open alerts: 0  code scanning open: 0   secret scanning alerts: 0
default_workflow_permissions: read
```

Every action is pinned to a full SHA. Workflows run on `pull_request`, not
`pull_request_target`, with `contents: read`; only `publish` takes
`id-token: write`, and only on a push to `master` after `test` and `e2e`.

**No secrets in history or in the shipped bundle**, checked with a positive
control so that the zero means something:

```
git grep -InE '(AKIA…|BEGIN …PRIVATE KEY|ghp_…|GOCSPX-…|client_secret\s*[:=]…)' $(git rev-list --all) -- .
(no output)

git grep -c 'snippet_app' $(git rev-list --all) -- . | head -3
ab84f39…:.env.example:2        <- control hit: the search works

grep -oE '(AIza…|client_secret|GOCSPX…|AKIA…)' <the live bundle>
(no output)
```

`.env` and `.kubeconfig-eks` are `0600` and untracked; Terraform state is
gitignored. The state bucket shows versioning, SSE-KMS with a customer key, all
four public-access blocks, `BucketOwnerEnforced`, and a `DenyInsecureTransport`
policy.

### Dependency and image scanning (clean)

```
make vuln            No vulnerabilities found.        (govulncheck 1.10.0)
npm audit            found 0 vulnerabilities
trivy, deployed api  debian 12.15: 0 · godough: 0 · migrate: 0
trivy, deployed web  alpine 3.24.2: 0
```

## Reproducing the evidence

Two of the commands above need an exact flag; the rest are in `SEC-TODO.md` next
to the item they close.

```
openssl s_client -tls1 -connect godough.foonix.net:443 -servername godough.foonix.net -cipher 'DEFAULT@SECLEVEL=0:ALL'
```

A modern client refuses TLS 1.0 on its own and reports "no protocols available",
so without `@SECLEVEL=0` the server looks like it is refusing. `nmap --script
ssl-enum-ciphers` reaches the same conclusion without the trick and is the better
citation.

```
gh api repos/saxeum/godough/rules/branches/master
gh api repos/saxeum/godough/branches/master/protection
```

The first lists active rules and the second lists required checks plus, by its
absence, the review requirement. Neither alone tells the whole story.
