# Staging data stack

This stack creates the staging VPC, the NAT gateway the EKS nodes reach the
internet through, the SSM bastion, and the private RDS Postgres instance. The NAT
is one gateway in one AZ, about $37/month with its address; the reasoning is
beside it in `main.tf`.

```
cp backend.hcl.example backend.hcl
# set bucket to the #102 output
terraform init -backend-config=backend.hcl
terraform plan -out=tfplan
terraform apply tfplan
```

## Break-glass database access

The bastion is SSM Session Manager only: no inbound rules, no SSH. You need the
[AWS Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
and valid AWS credentials.

From the repository root, use the two Make targets:

```
make rds-tunnel
```

Then in another terminal:

```
make rds-psql
```

`rds-psql` accepts `PSQL_ARGS`, for example `make rds-psql PSQL_ARGS='-c "select 1"'`.

The equivalent manual commands, run from this directory, are below.

Download the RDS CA bundle once. It is gitignored.

```
curl -o global-bundle.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
```

Open the tunnel in one terminal:

```
aws ssm start-session --region ap-southeast-2 \
  --target "$(terraform output -raw bastion_instance_id)" \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters host="$(terraform output -raw rds_endpoint | cut -d: -f1)",portNumber="5432",localPortNumber="15432"
```

Connect in another terminal. `host` is the RDS endpoint so `verify-full` checks
the certificate; `hostaddr` is the tunnel.

```
SECRET_ARN="$(terraform output -raw master_user_secret_arn)"
PGPASSWORD="$(aws secretsmanager get-secret-value --region ap-southeast-2 --secret-id "$SECRET_ARN" --query SecretString --output text | jq -r .password)" \
psql "host=$(terraform output -raw rds_endpoint | cut -d: -f1) hostaddr=127.0.0.1 port=15432 dbname=godough user=godough sslmode=verify-full sslrootcert=global-bundle.pem"
```

Expected: `SSL Connection: true`, TLSv1.3. The password comes from the
RDS-managed Secrets Manager secret and is never written to a file.

Plaintext is rejected by `rds.force_ssl=1`:

```
psql "host=$(terraform output -raw rds_endpoint | cut -d: -f1) hostaddr=127.0.0.1 port=15432 dbname=godough user=godough sslmode=disable"
```

The long-term access path is an EKS pod or Job in the private subnets; the
bastion is for break-glass work.

## The application role

`godough_app` is created once per cluster, before the first migration, by
`db/bootstrap.sql`. The SQL is the same file the test database uses; only the
password differs, and it arrives as a psql variable rather than in the file.

```
make rds-app-password
```

Creates `godough-staging/app-password` in Secrets Manager, a JSON value of
`{"password": "..."}` with 24 random bytes in hex. Hex on purpose: the password
goes into a connection URI, and hex needs no percent-encoding. **Terraform does
not create this secret**, so the password is not in state, in a plan file, or in
this repository. Re-running changes nothing; rotation is an explicit
`aws secretsmanager put-secret-value`, never a side effect of `terraform apply`.

```
make rds-tunnel    # in one terminal
make rds-bootstrap # in another
```

`rds-bootstrap` connects as the master, through the tunnel, and runs the same
file with the password from the secret. It is idempotent, and a re-run after a
rotation puts the database back in line with the secret rather than silently
keeping the old password.

Local development is unchanged: `make db-bootstrap` passes no variable, so the
development default in the file applies.

## The DSN the application uses

Not stored: assembled where it is needed from the endpoint and the password, so
there is one secret rather than two copies of one.

```
postgres://godough_app:<password>@<rds_endpoint>/godough
  ?sslmode=verify-full&sslrootcert=/certs/global-bundle.pem&pool_max_conns=5
```

`sslrootcert` is the in-cluster path, where the chart mounts the CA bundle
(#125). `pool_max_conns=5` is pinned because `pgxpool`'s default is
`max(4, NumCPU)` per replica, and a default `max_connections` of 100 across ten
replicas is a connection storm waiting for a rollout.

`godough_app` owns nothing, so any schema change is still the owner's, through the
migration Job (#115).
