# Staging app stack

The account-side half of the deployment: the edge certificate, the ALB log bucket,
the WAF, and the Argo CD installation itself. The two workloads are **not** applied
here any more — Argo owns them, from `deploy/argocd/` (#127).

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
terraform plan -var alb_hostname=k8s-godough-7d3b96a5d4-386819741.ap-southeast-2.elb.amazonaws.com
```

`alb_hostname` has to be passed **every time**. It has an empty default, and
`cloudflare_dns_record.edge` is `count = var.alb_hostname == "" ? 0 : 1`, so a plan
without it wants to delete the record that points the domain at the ALB; a plan
with a `will be destroyed` line for `cloudflare_dns_record` is that mistake and not
a change. The value is the load balancer's DNS name, which the Ingress creates.

There are no image digests on the command line any more: the digest a workload runs
is in its Application under `deploy/argocd/`, which is the object a reviewer reads
and the object Argo applies. `make image-digests` still prints the newest published
tag and digests, and it is not the same as `origin/master` — a docs-only merge
publishes nothing, so `HEAD` frequently has no image.

After the first apply of a new ACM certificate, the Ingress in Argo refers to it by
ARN, so a certificate replacement is two steps: apply here, then update the
`certificateArn` in the Applications.

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

### Sign-in on the edge

```
make k8s-secrets
```

also writes `godough-oauth` from `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` in
the environment, with `GOOGLE_REDIRECT_URL` built from `EDGE_URL`
(`https://godough.foonix.net/api/auth/google/callback` by default). The chart
references that Secret with `optional: true` on each key, so a cluster without
sign-in credentials boots with sign-in disabled rather than failing to start.

Two things have to agree with Google's console, and only one of them is in this
repository:

1. The **authorised redirect URI** must contain exactly
   `https://godough.foonix.net/api/auth/google/callback`. If it does not, the
   flow fails after the user has already consented, on Google's page, with
   `redirect_uri_mismatch`.
2. `TRUSTED_ORIGINS` must contain `https://godough.foonix.net`, or the API's CSRF
   check rejects the callback. It is `trusted_origins` in the stack, and it does
   not accept wildcards.

```
curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' https://godough.foonix.net/api/auth/google
```
`302 https://accounts.google.com/o/oauth2/v2/auth?...` means the client ID and
secret reached the pod. `503 {"error":"sign-in is not configured"}` means they
did not — that is the server saying the Secret is missing or incomplete, not a
problem with the redirect URI.

### Verified on the live edge

```
dig godough.foonix.net          -> CNAME k8s-godough-7d3b96a5d4-...elb.amazonaws.com
https  /                        -> 200, certificate verified by curl
http   /                        -> 301 to https
https  /api/snippet             -> 401 {"error":"Unauthorized"}      # the API answered
https  /healthz                 -> 404
```

The 401 is the pass condition for routing: the api's `/api` path is evaluated
before the ui's `/`, so the request reached the API and its auth, not the SPA's
index.html. The 404 on `/healthz` is also correct — that path is deliberately
absent from the Ingress, because the target group health-checks the pod directly
and a 200 served by the SPA would be a health check that passes while the API is
dead.

## What is not here

The Ingress/ALB (#126), the migration Job and its ordering (#115), and anything
Argo (#127). The namespace is created by the api release because there are two
workloads in it, which is the one thing a namespace-level release could own.
