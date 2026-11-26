# aws-test

Checks the Terraform bootstrap resource in the live AWS account:

- the state bucket exists and is readable;
- all four S3 public-access blocks are enabled;
- the bucket policy status is not public;
- no ACL grant targets `AllUsers` or `AuthenticatedUsers`;
- an anonymous HTTPS GET of the bucket root returns `403`, not a listing.

It is a separate Go module so the application does not link the AWS SDK.

```
make aws-test
```

or:

```
cd infra/aws-test
go test -count=1 -v ./...
```

The bucket name defaults to `godough-terraform-state-<account-id>` from STS. Set `STATE_BUCKET` to override it, and `AWS_REGION`/`AWS_DEFAULT_REGION` if the default `ap-southeast-2` is wrong. It uses the normal AWS credential chain, so an active SSO session is enough.
