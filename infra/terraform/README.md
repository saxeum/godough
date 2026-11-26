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
