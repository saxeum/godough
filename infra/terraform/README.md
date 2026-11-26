# Terraform

Two stacks, in order:

1. `bootstrap/` creates the S3 bucket that holds the state for everything else. Its own state is local and ignored by git; it cannot live in the bucket it creates.
2. The AWS application stacks (ECR, EKS, RDS) use that bucket with S3 native locking (`use_lockfile = true`).

## Bootstrap

```
cd infra/terraform/bootstrap
terraform init
terraform validate
terraform plan -out=tfplan
terraform apply tfplan
```

The bucket is named `godough-terraform-state-<account-id>`, versioned, encrypted with SSE-S3, public access blocked, and denies any request whose `aws:SecureTransport` is false. Read the outputs with:

```
cd infra/terraform/bootstrap && terraform output
```

The next stack supplies the bucket and region through a `backend.hcl`, not through committed account-specific values.

## ECR and the publish role

`ecr/` creates `godough-api` and `godough-web` with immutable tags, scan-on-push, AES256 and lifecycle rules, plus the GitHub OIDC provider and the `godough-github-publish` role.

```
cd infra/terraform/ecr
cp backend.hcl.example backend.hcl
# set bucket to the #102 output
terraform init -backend-config=backend.hcl
terraform plan -out=tfplan
terraform apply tfplan
```

Outputs:

- `registry` — `<account>.dkr.ecr.<region>.amazonaws.com`
- `repository_urls` — api and web
- `github_actions_role_arn` — set this as the `AWS_ROLE_ARN` repository variable so `.github/workflows/ci.yml` can assume it through OIDC

## Building and pushing

```
make ecr-push
```

`ECR_REGISTRY` is derived from STS when unset. `TAG` overrides both; otherwise
`ecr-push` uses the current commit and `ecr-pull-test` uses `origin/master`.

`make ecr-build` and `make ecr-login` are separate steps. The tag is the commit SHA; there is no `latest`. The CI `publish` job runs the same `make ecr-push` after `test` and `e2e`, only on `master`, using OIDC and the role above.

```
make ecr-pull-test
```

pulls both images back, starts the compose stack from them with `docker-compose.ecr.yml`, and runs `make e2e`.

## Network and RDS

`data/` creates the staging VPC and database:

- two public subnets for a future internet-facing ALB;
- two private subnets for RDS and the future EKS node group;
- an EKS node security group with no ingress, which the database trusts on 5432;
- RDS Postgres `17.12`, `db.t4g.micro`, single-AZ, 20 GiB gp3, encrypted;
- `rds.force_ssl=1`, CA `rds-ca-rsa4096-g1`, `publicly_accessible=false`;
- RDS-managed master password in Secrets Manager, so Terraform never sees it.

```
cd infra/terraform/data
cp backend.hcl.example backend.hcl   # set bucket to the #102 output
terraform init -backend-config=backend.hcl
terraform plan -out=tfplan
terraform apply tfplan
```

Outputs include `rds_endpoint`, `rds_port`, `rds_ca_cert_identifier`,
`master_user_secret_arn`, `vpc_id`, the subnet ids and
`eks_node_security_group_id`.

The database is private. A `verify-full` connection is a staging/EKS exercise,
not something a host outside the VPC can run; `make tls-check` should be run
from the EKS node or a job inside the VPC once the cluster exists.
