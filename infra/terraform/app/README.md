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
| `godough-api` | `DATABASE_URL` | `make k8s-secrets`, which reads `godough-staging/app-password` |
| `godough-rds-ca` | `global-bundle.pem` | `make k8s-secrets`, from `global-bundle.pem` in the repo root |

```
make k8s-secrets
```

Creates both, and the namespace. It is imperative rather than synced: External
Secrets or the Secrets Store CSI driver would keep them current by themselves, and
each costs an operator, a CRD and an IRSA role. For one consumer and one operator,
a command that has to be re-run after a rotation is the smaller thing; that
trade-off is worth revisiting when a second workload needs the same credential.

It also refuses to run without `make eks-tunnel`, because a `kubectl` that cannot
reach the API and a cluster that is down look identical from the outside.

`wait = true` on both releases is deliberate: applied means running, not merely
submitted. A missing Secret therefore fails the apply after the timeout instead
of reporting success over a CrashLoopBackOff.

## The edge: ALB through Cloudflare

Two pieces, and the second one cannot be finished by Terraform alone.

**The controller.** `aws-load-balancer-controller`, installed here with an IRSA
role and upstream's policy vendored at `policies/`. It is what turns an Ingress
into an ALB whose target groups hold **pod IPs** — an `aws_lb` built directly
would route to the node and lose the pod, which is what per-pod health checks and
readiness need. It discovers the public subnets by tag, which the data stack sets
(`kubernetes.io/role/elb` and the cluster tag).

**The certificate.** `aws_acm_certificate.edge` for `godough.foonix.net`,
regional, because the ALB terminates TLS. It is created but **not validated**:
the validation CNAME lives in the Cloudflare zone and this configuration has no
Cloudflare credentials. Add the record, then the second apply can use the ARN:

```
terraform output -json certificate_validation_record
```

**The DNS records, both in Cloudflare.**

1. The ACM validation CNAME from the output above. One record, and it only has to
   exist until the certificate is issued.
2. `godough.foonix.net` → the ALB's DNS name, once the Ingress has created it.
   Use a CNAME, and whether it is proxied is a real decision:
   - **DNS only (grey):** clients talk to the ALB, so the ALB's certificate is
     what browsers see. Simplest, and what to do first.
   - **Proxied (orange):** Cloudflare terminates TLS and connects to the ALB. Set
     the SSL mode to Full (strict) so the ALB's certificate is still verified;
     Flexible would send plaintext across the internet.

When CloudFront arrives it takes the orange path's place: the record points at the
distribution, and CloudFront needs **its own certificate in `us-east-1`** — a
regional ACM certificate cannot be attached to a distribution. That is why the
certificate here is explicitly regional.

## What is not here

The Ingress/ALB (#126), the migration Job and its ordering (#115), and anything
Argo (#127). The namespace is created by the api release because there are two
workloads in it, which is the one thing a namespace-level release could own.
