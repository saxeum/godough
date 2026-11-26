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
ECR_REGISTRY=<registry> TAG=$(git rev-parse HEAD) make ecr-push
```

`make ecr-build` and `make ecr-login` are separate steps. The tag is the commit SHA; there is no `latest`. The CI `publish` job runs the same `make ecr-push` after `test` and `e2e`, only on `master`, using OIDC and the role above.
