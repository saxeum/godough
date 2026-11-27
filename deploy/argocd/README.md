# Argo CD applications

An app-of-apps: one root Application is hand-applied once, and everything below it —
the namespace, the ApplicationSet, the two workloads — arrives through sync. Adding a
third workload is a line in `clusters/staging/charts.yaml` and a merge.

```
deploy/argocd/
  install/            the argo-cd install: values the Terraform release uses
  root/               application.yaml — the one object applied by hand
  clusters/staging/   namespaces.yaml, charts.yaml — what this cluster targets
deploy/charts/*/values-staging.yaml   the deployment's values, beside each chart
```

## Bootstrap

```
make eks-tunnel        # one terminal, the endpoint is private
make argocd-seed       # the other: applies deploy/argocd/root/
```

`make argocd-apps` then shows `root`, `godough-api` and `godough-ui`. Re-running
`argocd-seed` is harmless; it is the same `kubectl apply` that created the root.

That is the one documented exception to "nothing applied outside the pipeline", and it
exists because an app-of-apps needs exactly one object placed from outside. Everything
after it is sync.

## Looking at it

Two terminals, because the forward blocks:

```
make eks-tunnel
make argocd-forward        # http://127.0.0.1:8090
make argocd-password       # user admin; the generated password, until someone changes it
```

`make argocd-login` prints that password and runs `argocd login … --insecure`, which
prompts — deliberately not an argument, which `ps` would show. `make argocd-apps` is
the state without the UI or the CLI at all.

## Sync policy, and where it differs

| | automated | prune | why |
|---|---|---|---|
| `root` | yes | yes | it syncs *definitions* — namespaces, the ApplicationSet. A definition deleted from git should stop existing. |
| the workloads | yes | yes | one environment, and merge is the approval. There is no human-sync step to protect. |

`root` and the workloads differ on one thing only: the workloads' prune is what makes
a bad values key able to delete an Ingress, so the first weeks of this are worth
watching rather than trusting. If godough ever grows a second environment, the
ghetto-kube split applies: root automated in both, workloads manual in the one that is
not staging.

## Verifying

```
make argocd-verify
```

Fails when any Application is not `Synced`/`Healthy`, or when a pod is running an
image digest the values file does not name. Both, because either alone is a green
light that lies: `Synced` is Argo's opinion about the objects it manages, and an image
hand-patched onto a Deployment is drift Argo will revert *later*. The digest check
compares every replica, not the first — a half-finished rollout runs two digests at
once.

## Values

`values-staging.yaml` beside each chart, referenced with `valueFiles`. Argo resolves
value files relative to the chart path, so a chart in this repository needs no
`$values` ref source — that machinery is for charts that live somewhere else.

The digest in that file is the digest that runs. A promotion is an edit to it, a pull
request, and a merge; Argo reconciles. There is no `-var`, no tunnel and no laptop in
that path.

## Not here, and why

- **No ingress for Argo.** An exposed control plane is a much larger surface than a
  port-forward, and there is one human.
- **Argo does not upgrade itself.** Terraform installs it. An Argo that manages its own
  installation cannot be debugged by the thing it manages when it breaks; that is worth
  revisiting when there is a second cluster to keep in step.
- **No cascade finalizer on `root`.** With prune on, the two agree, and deleting the
  root would delete every object it owns. That should be a deliberate act; it gets
  added in its own commit once the tree has been stable.
