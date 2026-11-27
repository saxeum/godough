# Argo CD applications

The two Applications that own what runs in the `godough` namespace. Terraform
installs Argo itself (`helm_release.argocd`, `infra/terraform/app`) and the cluster
around it; from here on, git is the only writer of the workloads.

## Bootstrap

The Applications are applied once, by hand, because an app-of-apps needs one root
Application to point at the rest and that root is exactly this directory:

```
kubectl apply -f deploy/argocd/
```

That is the one documented exception to "nothing applied outside the pipeline", and
it is why the Applications are a directory rather than something Terraform builds.

## Looking at it

Argo has no ingress. Port-forward and log in with the generated admin secret:

```
kubectl -n argocd port-forward svc/argocd-server 8080:80
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
```

The initial password is only there until someone sets one; it is a bootstrap
credential, not a secret to keep.

## What is in an Application, and why it is inline

Values are `helm.valuesObject` in the Application, not a `valueFiles` reference.
Argo resolves value files relative to the chart directory, and this deployment's
values are not the chart's — the chart's `values.yaml` is its schema and defaults.
Keeping them inline also means the object that is reviewed is the object that is
deployed: **the digest in git is the digest running**, which is the property #127
exists to buy and which Terraform state alone never gave us.

## Promotion

A new image reaches the cluster by editing `image.digest` in the Application for
that workload and merging. Argo reconciles. There is no `-var`, no tunnel and no
laptop in the path.

## Turning sync on

`automated` is deliberately absent until the `helm_release` resources are gone from
`infra/terraform/app` — two controllers describing the same Deployment is how a
rollout becomes a fight. The commit that removes them is the commit that enables
`automated` with `prune: false`, and prune stays off until Argo has been the only
writer for long enough to trust it.
