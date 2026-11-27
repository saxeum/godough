# Staging app stack

Installs the two charts from `deploy/charts` into the cluster from the `eks`
stack. Argo CD replaces this later (#127); until then `terraform apply` is the
deploy.

```
cp backend.hcl.example backend.hcl   # set bucket to the #102 output
terraform init -backend-config=backend.hcl
```

## Applying it

```
make eks-tunnel
```

Leaving that running in its own terminal, in another:

```
make image-digests
```

```
terraform plan -var image_registry=<registry> -var api_digest=<sha256:...> -var ui_digest=<sha256:...>
```

Both digests are **required variables with no default**. A tag can move between
review and rollout, so the digest is a decision to make and write down, not
something to fall back on. `make image-digests` prints the newest *published*
tag and digests, which is not the same as `origin/master`: a docs-only merge
publishes nothing, so `HEAD` frequently has no image.

The provider reads the kubeconfig `make eks-kubeconfig` wrote, which dials
`127.0.0.1:6443` with the real hostname as `tls-server-name`. Without
`make eks-tunnel` the plan fails to reach the API — that is the private endpoint
doing its job, not a misconfiguration.

## Prerequisites, and why apply fails without them

Two Secrets must exist before the api pod can start. `terraform apply` does not
create them and must not: one holds a database password, the other a CA bundle.

| Secret | Key | From |
|---|---|---|
| `godough-api` | `DATABASE_URL` | `make rds-app-password`, then the synced DSN (#125 decides the sync) |
| `godough-rds-ca` | `global-bundle.pem` | the RDS global bundle, `global-bundle.pem` in the repo root |

`wait = true` on both releases is deliberate: applied means running, not merely
submitted. A missing Secret therefore fails the apply after the timeout instead
of reporting success over a CrashLoopBackOff.

## What is not here

The Ingress/ALB (#126), the migration Job and its ordering (#115), and anything
Argo (#127). The namespace is created by the api release because there are two
workloads in it, which is the one thing a namespace-level release could own.
