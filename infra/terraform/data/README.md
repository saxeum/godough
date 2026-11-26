# Staging data stack

This stack creates the staging VPC, the SSM bastion, and the private RDS
Postgres instance.

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
and valid AWS credentials. Run the commands below from this directory.

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
bastion is for break-glass work. The app role `godough_app` still needs the
bootstrap/migration step against this database.
