# Deploy charts

One release per workload. Terraform installs them (#125); Argo CD replaces
Terraform later (#127). Nothing here is applied by hand.

```
make charts-lint
```

lints both charts, renders them with a placeholder digest, and checks that they
still refuse to render without one. CI runs the same target.

## The contract with the rest of the deploy

**Images are digests.** `image.digest` has no default and the chart fails without
it, because a tag can move between review and rollout. Terraform passes the digest
that CI published.

**Secrets are created outside the chart**, from Secrets Manager, by #114. The
chart only references them, so a password never appears in a values file or in a
release's history:

| Secret | Key | Used by |
|---|---|---|
| `godough-api` | `DATABASE_URL` | api, the application role's DSN with `sslmode=verify-full&sslrootcert=/certs/global-bundle.pem` |
| `godough-api` | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL` | api, all optional and all-or-none |
| `godough-rds-ca` | `global-bundle.pem` | api, mounted at `/certs` |

`pool_max_conns` belongs in that DSN, not in the chart: replication multiplies
connections, and a pool of `max(4, NumCPU)` per replica reaches a default
`max_connections` of 100 quickly.

**Probes are split on purpose.** Liveness is `/healthz`, which answers without the
database; readiness is `/readyz`, which pings it. Verified against the real image:
with Postgres stopped, `/healthz` stays `200` and `/readyz` returns `503`, so a
database outage removes the pod from the Service instead of restarting it forever.

The ui probes `/`, never `/healthz`: the container answers `404` there on purpose
so that index.html can never satisfy a health check.

## What is not here yet

The Ingress/ALB (#126), the migration Job's ordering (#115), and the Terrafom
`helm_release` resources that apply these.
