# Staging EKS stack

One EKS control plane and one managed node group, enough to run the API, the UI
and the migration Job from #115.

```
cp backend.hcl.example backend.hcl   # set bucket to the #102 output
terraform init -backend-config=backend.hcl
terraform plan -out=tfplan
terraform apply tfplan
```

Apply the data stack first: this stack reads the VPC, private subnet and node
security group ids from its state.

## What it creates

- **Control plane**, Kubernetes `1.37`, private endpoint only, admin access for
  the identity that applies it.
- **One managed node group**: Graviton `t4g.small`, min 1, max 2, in the private
  subnets. The data stack's `godough-eks-nodes` security group is attached through
  a launch template, which is the only way to attach an extra group to a managed
  node group — RDS already trusts that group on 5432.
- **Add-ons**: VPC CNI, kube-proxy, CoreDNS, and the EBS CSI driver. Version
  resolution is `most_recent` for the cluster's Kubernetes version, so an EKS
  upgrade does not leave an add-on pinned to an unsupported version. The CSI
  driver is the one add-on with its own role.
- **OIDC provider** for IRSA, and the EBS CSI role that uses it. #114 and #115
  add their own roles the same way.
- **A KMS key** for Kubernetes Secret encryption in etcd. Not in #117's list, but
  this is where the database credentials land and Trivy's config gate fails a
  cluster without it. It cannot be removed later.

Not created, deliberately: Fargate, Auto Mode, a service mesh, Karpenter, a
cluster autoscaler, and any public ingress. Each is a mechanism without a
workload that needs it yet.

The NAT gateway is in the data stack with the rest of the network; see
[../data/README.md](../data/README.md).

## Accessing the cluster

The API endpoint is private, like the database. `aws eks update-kubeconfig` alone
gets you a kubeconfig that cannot connect from a laptop; the tunnel is the same
SSM path `make rds-tunnel` uses for psql.

### Why the same instance as RDS, and what it must not become

It is one host, and it stays a pipe. Nothing is installed on it, it holds no
kubeconfig, and the instance role is not added to the cluster's access entries,
so the forward carries bytes and nothing else. Authentication happens as *your*
identity against the cluster's access entries, which is the point: CloudTrail and
the EKS audit log name the person, not an instance role that everybody shares.
The alternative — `kubectl` on the box, or an access entry for its instance
profile — would turn "database break-glass" into "cluster admin shell", and
after #114/#115 the cluster holds the database credentials.

Two consequences worth knowing. The tunnel does not grant access: it exposes 443
of the cluster endpoint, which still refuses an unauthenticated caller. And if
the forward connects but `kubectl` hangs or resets, the thing to check is whether
the cluster's security group allows 443 from the bastion's security group.

Deploys do not go through a laptop. The charts are installed by Terraform (#125)
and, after that, by Argo CD (#127), which runs inside the cluster and so reaches
the private endpoint without a forward at all. This tunnel is for the smoke test
(#124) and for debugging: `get`, `logs`, `describe`, `exec`.

The open question is the Helm provider, not the human: if it runs from outside the
VPC it needs this same forward, and #125 decides whether that is acceptable or
whether the provider reaches the API from inside the cluster.

Start the forward in its own terminal and leave it running:

```
make eks-tunnel
```

In a second terminal:

```
make eks-kubeconfig
```

That merges a `godough-staging` context into `~/.kube/config` and makes it
current, so `kubectl` works with nothing exported — the previous version wrote a
separate file and printed an `export` line, which is a step people skip and then
see `dial tcp localhost:80`, the error you get from having no kubeconfig at all.

`aws eks update-kubeconfig` merges rather than replacing, so other contexts
survive; changing the current context is the one side effect. For no side effects
at all:

```
EKS_KUBECONFIG=/tmp/godough-kubeconfig make eks-kubeconfig
```

The context dials `127.0.0.1:6443` with the real hostname as `tls-server-name`, so
the certificate validates over the forward. The target ends by saying whether the
server actually answers, because a context pointing at a port with no forward
behind it is otherwise indistinguishable from a broken cluster:

```
server: NOT reachable. The endpoint is private: run 'make eks-tunnel' in another terminal.
```

## Smoke test

Run this after the first apply and before any chart exists. It answers four
questions: can I reach the API, is the node usable, can a pod pull the project's
own image from ECR, and does a volume actually provision — which is the only
end-to-end proof that the IRSA role works.

None of these commands have been run against a live cluster: the stack was
planned and validated, not applied. The first apply is what makes them checkable,
and the expected output below is what should appear, not what has been seen.

### Reach the API

With `make eks-tunnel` running in its own terminal, and `KUBECONFIG` exported:

```
kubectl version -o yaml
```

The server block has a `gitVersion` and a `kustomizeVersion`. No server block
means the tunnel or the TLS server name is wrong.

```
kubectl auth can-i '*' '*'
```

Prints `yes`: the identity that applied the cluster is an access entry admin.

```
kubectl get nodes -o wide
```

One node, `Ready`, `v1.37.x`, `INTERNAL-IP` in `10.42.0.0/16`, `EXTERNAL-IP`
`<none>`. A public IP here means the node group landed in the wrong subnets.

```
kubectl get pods -A
```

`aws-node`, `kube-proxy` and `ebs-csi-node` Running on the one node, `coredns`
with two ready replicas, `ebs-csi-controller` with its containers Running.

```
aws eks describe-cluster --region ap-southeast-2 --name godough-staging --query 'cluster.resourcesVpcConfig.endpointPublicAccess'
```

Prints `false`. If this ever says `true`, someone re-enabled the public endpoint
and the tunnel is not what is protecting the API.

Negative control — stop the tunnel with `^C`, then:

```
kubectl get nodes --request-timeout=5s
```

Fails with `connect: connection refused` on `127.0.0.1:6443`. That is the check
that the kubeconfig is talking to the forward and not to something else.

### Pull an image from ECR

```
export REGISTRY=$(terraform -chdir=infra/terraform/ecr output -raw registry)
```

```
export TAG=$(git rev-parse origin/master)
```

```
kubectl run ecr-pull --image=$REGISTRY/godough-api:$TAG --restart=Never -- /godough
```

```
kubectl describe pod ecr-pull | grep -E 'Pulled|Started|Failed'
```

`Pulled` and `Started` are the pass. The container then exits non-zero because
`SNIPPET_ADDR` and `DATABASE_URL` are unset — that is the api refusing to start,
not a pull failure. `ErrImagePull` or `401` means the node role cannot read the
repository.

```
kubectl delete pod ecr-pull
```

### Resolve DNS and reach the internet

```
kubectl run dns-check --rm -i --restart=Never --image=busybox:1.37 -- nslookup kubernetes.default
```

Answers with the `kubernetes.default` service address in the service CIDR. A
timeout here is CoreDNS; a pull failure is the NAT gateway.

### Provision a volume

The EBS CSI controller is the only workload using IRSA, so a bound volume is the
only proof that the OIDC provider, the trust policy and the role are all right.

```
cat > /tmp/smoke-ebs.yaml <<'YAML'
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: smoke-gp3
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
reclaimPolicy: Delete
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: smoke-claim
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: smoke-gp3
  resources:
    requests:
      storage: 1Gi
YAML
```

```
kubectl apply -f /tmp/smoke-ebs.yaml
```

```
kubectl get pvc smoke-claim -o wide
```

`Bound`, and the `VOLUME` column holds a `pvc-…` id.

```
kubectl describe pvc smoke-claim | grep -i 'provisioner\|ebs.csi.aws.com'
```

Names `ebs.csi.aws.com`. A PVC stuck in `Pending` with `failed to provision
volume` in `kubectl describe pvc` is the IRSA trust, not storage.

```
kubectl delete -f /tmp/smoke-ebs.yaml
```

Deletes the claim, the volume and the class together; `reclaimPolicy: Delete` is
what stops 1 GiB of gp3 sitting in the account afterwards.

### What this does not cover

Reaching RDS from a pod with `verify-full` and the CA bundle is #116, and needs
the owner and application secrets from #114. Nothing here writes to the database.

## Cost

Approximate, `ap-southeast-2`, excluding data transfer: control plane ~$73/month,
one `t4g.small` node ~$12–15, the NAT gateway and its address ~$37, the KMS key
~$1. With the data stack's RDS and bastion, staging costs roughly $140/month to
sit idle.

## Why not Fargate, why not Auto Mode

Fargate bills per pod with a minimum, cannot run a DaemonSet (so no node-level log
or metrics agent), and needs a second pod-level networking and security-group
model for the same two workloads. Auto Mode is simpler than Karpenter but less
transparent than a node group: when a pod does not schedule, a managed node group
gives you a node to look at.
