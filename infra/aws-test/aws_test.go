// Package awstest checks the Terraform bootstrap resources in the live AWS
// account. It is a separate module so the application's dependency graph does
// not grow the AWS SDK; run it from this directory.
package awstest

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	defaultRegion = "ap-southeast-2"
	bucketPrefix  = "godough-terraform-state-"
)

// TestStateBucketIsDeployedAndPrivate is the acceptance check for #102. It
// proves the bucket exists, has all four public-access blocks, has a
// non-public policy status, and has no public ACL grants.
func TestStateBucketIsDeployedAndPrivate(t *testing.T) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region()))
	if err != nil {
		t.Fatalf("load AWS config: %v", err)
	}

	bucket := os.Getenv("STATE_BUCKET")
	if bucket == "" {
		account, err := accountID(ctx, cfg)
		if err != nil {
			t.Fatalf("derive bucket name: %v", err)
		}
		bucket = bucketPrefix + account
	}

	s3Client := s3.NewFromConfig(cfg)

	if _, err := s3Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket}); err != nil {
		t.Fatalf("state bucket %q is not deployed or not readable: %v", bucket, err)
	}

	access, err := s3Client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: &bucket})
	if err != nil {
		t.Fatalf("get public access block for %q: %v", bucket, err)
	}
	if access.PublicAccessBlockConfiguration == nil {
		t.Fatalf("state bucket %q has no public access block", bucket)
	}
	pab := access.PublicAccessBlockConfiguration
	for name, value := range map[string]*bool{
		"BlockPublicAcls":       pab.BlockPublicAcls,
		"IgnorePublicAcls":      pab.IgnorePublicAcls,
		"BlockPublicPolicy":     pab.BlockPublicPolicy,
		"RestrictPublicBuckets": pab.RestrictPublicBuckets,
	} {
		if value == nil || !*value {
			t.Errorf("state bucket %q: %s is not true", bucket, name)
		}
	}

	policyStatus, err := s3Client.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{Bucket: &bucket})
	if err != nil {
		t.Fatalf("get policy status for %q: %v", bucket, err)
	}
	if policyStatus.PolicyStatus == nil || policyStatus.PolicyStatus.IsPublic == nil {
		t.Fatalf("state bucket %q has no policy status", bucket)
	}
	if *policyStatus.PolicyStatus.IsPublic {
		t.Errorf("state bucket %q has a public bucket policy", bucket)
	}

	acl, err := s3Client.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: &bucket})
	if err != nil {
		t.Fatalf("get ACL for %q: %v", bucket, err)
	}
	for _, grant := range acl.Grants {
		if grant.Grantee == nil || grant.Grantee.URI == nil {
			continue
		}
		if isPublicGroup(*grant.Grantee.URI) {
			t.Errorf("state bucket %q grants access to public group %s", bucket, *grant.Grantee.URI)
		}
	}

	t.Logf("state bucket %s exists and is not public", bucket)
}

func region() string {
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}
	return defaultRegion
}

func accountID(ctx context.Context, cfg aws.Config) (string, error) {
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	if out.Account == nil || *out.Account == "" {
		return "", errors.New("AWS returned no account id")
	}
	return *out.Account, nil
}

func isPublicGroup(uri string) bool {
	return strings.HasSuffix(uri, "/AllUsers") || strings.HasSuffix(uri, "/AuthenticatedUsers")
}
