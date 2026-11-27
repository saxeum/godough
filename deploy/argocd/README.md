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

## Sync policy

| | automated | selfHeal | prune | why |
|---|---|---|---|---|
| `root` | yes | yes | **yes** | it syncs *definitions* — a namespace or an ApplicationSet deleted from git should stop existing |
| the workloads | yes | yes | **no** | a definition is not a workload: an object that stops rendering because a values key was mistyped should not disappear and take the site with it |

Prune being off on the workloads is a decision with a cost, stated plainly: an object
removed from git keeps running until someone prunes it. That is the failure mode
chosen over its alternative — a typo deleting the Ingress, with the repository still
looking correct — because nothing runs `make argocd-verify` on a loop, and the cost of
the wrong object lingering is a diff that `make argocd-verify` or the next review will
show, while the cost of the wrong object vanishing is downtime.

The deliberate act is one command, per Application, with the diff available first:

```
make argocd-diff APP=godough-api
make argocd-prune APP=godough-api
```

Turning prune on in `clusters/staging/charts.yaml` is the thing to do when the values
have settled and someone is watching the syncs — not before.

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

## The migration Job

`migrations.enabled: true` renders a Helm hook Job, and Argo runs `pre-install,pre-upgrade`
hooks as **PreSync** — so it runs once per sync, including a sync where nothing changed.
Kept that way on purpose: #113's advisory lock plus the checksums make a second run a
sub-second no-op that exits cleanly, and the alternative (a separate Application with its
own lifecycle and a sync-wave to order it) is a mechanism with no present user. If sync
frequency ever makes a Job-per-sync expensive, that is the ticket.

## Not here, and why

- **No ingress for Argo.** An exposed control plane is a much larger surface than a
  port-forward, and there is one human.
- **Argo does not upgrade itself.** Terraform installs it. An Argo that manages its own
  installation cannot be debugged by the thing it manages when it breaks; that is worth
  revisiting when there is a second cluster to keep in step.
- **No cascade finalizer on `root`.** With prune on, the two agree, and deleting the
  root would delete every object it owns. That should be a deliberate act; it gets
  added in its own commit once the tree has been stable.
