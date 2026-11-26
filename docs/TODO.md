# Prioritised backlog

Reviewed against `origin/master` at `bea8ba4`, the merge of PR **#110**
(`infra/rds-vpc`, VPC + private RDS). That code is merged. Whether it is applied
in the account is not recorded in this repo, so check `terraform state list` in
`infra/terraform/data` before assuming a resource exists.

**The snippet app is the fixture; the platform is the product.** Priority goes
to work every future CRUD SaaS inherits: identity, tenancy, hermetic tests,
deployment, migrations, and only then edge polish. Snippet behaviour that is not
a reusable pattern does not earn a place in the queue.

## P0 — platform foundation — done

| Issue | Result |
|---|---|
| #45 | Dedicated `snippet_test` database; `make test` cannot touch the development database |
| #47 | `make lint`, `make test-race`, Vitest UI tests, CI gates |
| #44 | Structured logs, `X-Request-ID`, one access line per request |

## P1 — identity and tenancy — done

| Issue | Result |
|---|---|
| #52 + #55 | Postgres sessions and exact-Origin CSRF |
| #53 | Google OIDC flow with the offline `oidctest` seam |
| #54 | Ownership scoping and expand/contract migrations 0003/0004 |
| #56 | Sign-in UI, `/api/auth/session`, `/api/auth/signout` |
| #41 | Deferred until after EKS: rate limiting needs the deployed edge and the session key |

## P2 — deploy to AWS — in progress

| Order | Issue | State | Result / next |
|---|---|---|---|
| 1 | #102 | done | S3 Terraform state bucket, versioned, SSE-KMS, TLS-only; live `aws-test` |
| 2 | #103 + #104 | done | ECR repos, GitHub OIDC publish role, `make ecr-login/build/push/pull-test` |
| 3 | #38 | done | `verify-full` DSN template; `cmd/tlscheck` / `make tls-check` |
| 4 | #110 | done | Staging VPC, private RDS Postgres 17.12, SSM bastion; merged, applied state to be confirmed |
| 5 | #117 | next | EKS cluster with one normal managed node group |
| 6 | #5 | epic | Remaining deploy work: manifests, Ingress/ALB, secrets/IRSA, edge ownership |

#5 is the deployment epic, not a ticket to “do Kubernetes” in one pass. Its
remaining work is tracked by #117, #115, #116 and the edge decision below.

### EKS decision

Use a **normal managed node group**, not Fargate or Auto Mode, for staging:

- One EKS control plane.
- One Graviton `t4g.small` node group, min 1, max 2, in the private subnets.
- The existing `godough-eks-nodes` security group, so RDS needs no new ingress.
- One NAT gateway for ECR pulls and AWS APIs. It costs about the same as the
  interface endpoints it replaces, and is simpler.
- VPC CNI, kube-proxy, CoreDNS, EBS CSI, and an OIDC provider for IRSA.
- No Fargate, Auto Mode, service mesh, or Karpenter yet.

Fargate is not cheaper at this size, has per-pod minimums, no DaemonSets, and
needs a second pod-security-group model. Auto Mode is a reasonable later step;
it is not simpler to debug today. Approximate staging cost excluding data
transfer: control plane ~$73/mo, node ~$12–15, NAT ~$33, RDS ~$15, bastion ~$3.

## P2.1 — migrations — epic #111

Migrations are local-only today. Gaps:

- The API image contains `/migrate` but not `migrations/`.
- The runner has no advisory lock, no checksum, and no status command.
- RDS has no `godough_app` role; `db/bootstrap.sql` still has the dev password.
- There is no Kubernetes migration Job and no deploy ordering.
- CI publishes an image but never migrates the deployed database.

| Ticket | Work |
|---|---|
| #112 | Ship `migrations/` in the API image; the Job must use the same digest as the app |
| #113 | Advisory lock, checksums, and `-status`; keep forward-only semantics |
| #114 | Bootstrap `godough_app` on RDS from a Secrets Manager password |
| #115 | Kubernetes migration Job, same image digest, then deploy only after it completes |
| #116 | Prove the path against private RDS: role, `verify-full`, `schema_migrations`, `tls-check` |

Suggested order: **#112 → #113 → #117 → #114 → #115 → #116**. #112 and #113 are
local and unblocked; #114 can use the SSM bastion before #117 exists.

The API never migrates at startup and migrations are never an init container.
The Job runs in the VPC, not on a GitHub runner, because RDS is private.

## P3 — edge polish, after the topology decision

| Issue | Why deferred |
|---|---|
| #48 | Image size and the nginx-vs-distroless decision are moot if static assets move to S3/CDN or the ALB/Ingress owns the edge |
| #42 | Headers and CSP may belong to the edge, not the app |
| #43 | Compression may belong to the CDN/edge, not the app |

Do not implement #48/#42/#43 until #5 records which layer owns each behaviour.

## Deferred — snippet-domain work

- **#12** — expiry filtering. Snippet-domain logic.
- **#35** — pagination. The reusable list pattern matters, but not before the
  platform deploy path is complete.
- **#40** — content cap. The reusable part is request validation; revisit when a
  real app needs that pattern.

## Done

- **#34** — probes stay on the API pod.
- **#38** — TLS to Postgres with CA verification and expiry checking.
- **#44** — structured logs and request ids.
- **#45** — hermetic test database.
- **#46** — TypeScript generation delivered by #58.
- **#47** — lint, race and UI gates.
- **#52 + #55** — Postgres sessions and CSRF.
- **#53** — Google OIDC and the offline test seam.
- **#54** — ownership scoping and expand/contract migrations.
- **#56** — sign-in UI and session endpoints.
- **#59** — spec-first API epic.
- **#102** — Terraform state bucket and live AWS tests.
- **#103 + #104** — ECR repositories, OIDC publish, build/push/pull-test.

## Do not start yet

- Any next ticket before PR #118 (this backlog) is merged by the human.
- #115 or #116 before #117.
- #115 before #112 and #113.
- #48/#42/#43 before #5's topology decision.
- #12, #35 or #40 before the deploy path is complete.

## Working rules

- One ticket, one branch, one PR. Do not start the next one until the current
  one is merged and closed by the human.
- The author does not verify; the verifier does not merge.
- Acceptance evidence belongs in a comment with the SHA it ran against.
