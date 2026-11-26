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

A pipeline can open the same forward with its own OIDC credentials, which is an
alternative to a runner inside the VPC for #115. Whether it should, and with what
namespace-scoped RBAC rather than the publish role's cluster access, is #115's
decision and not settled here.

```
terraform -chdir=infra/terraform/data output -raw bastion_instance_id
```

```
terraform -chdir=infra/terraform/eks output -raw cluster_endpoint
```

Start the forward in its own terminal, with the bastion id and the endpoint host
from those two commands substituted:

```
aws ssm start-session --target <bastion-instance-id> --document-name AWS-StartPortForwardingSessionToRemoteHost --parameters '{"host":["<endpoint-host>"],"portNumber":["443"],"localPortNumber":["6443"]}'
```

Then point the cluster at the local port, keeping the TLS server name so the
certificate still validates:

```
aws eks update-kubeconfig --name godough-staging --region ap-southeast-2
```

```
kubectl config set-cluster "$(kubectl config view -o jsonpath='{.clusters[0].name}')" --server=https://127.0.0.1:6443 --tls-server-name=<endpoint-host>
```

```
kubectl get nodes -o wide
```

These three commands have not been run against a live cluster yet: the stack was
planned and validated, not applied. The first apply is what makes them checkable.

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
