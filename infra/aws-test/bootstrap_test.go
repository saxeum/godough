// Package awstest checks the Terraform bootstrap and ECR resources in the live
// AWS account. It is a separate module so the application's dependency graph
// does not grow the AWS SDK; run it from this directory.
package awstest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

const (
	defaultRegion = "ap-southeast-2"
	bucketPrefix  = "godough-terraform-state-"

	// publicBucketURL is a third-party AWS Open Data bucket with anonymous
	// list access. It is the positive control: the same GET that must be 403
	// on our state bucket must be 200 somewhere.
	publicBucketURL = "https://noaa-ghcn-pds.s3.amazonaws.com/"
)

type state struct {
	ctx    context.Context
	cfg    aws.Config
	bucket string
	region string
}

func loadAWS(t *testing.T) aws.Config {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(), config.WithRegion(awsRegion()))
	if err != nil {
		t.Fatalf("load AWS config: %v", err)
	}
	return cfg
}

func loadState(t *testing.T) state {
	t.Helper()

	ctx := t.Context()
	cfg := loadAWS(t)
	bucket, err := stateBucket(ctx, cfg)
	if err != nil {
		t.Fatalf("derive bucket name: %v", err)
	}
	return state{ctx: ctx, cfg: cfg, bucket: bucket, region: awsRegion()}
}

// TestStateBucketIsDeployedAndPrivate is the acceptance check for #102. It
// proves the bucket exists, has all four public-access blocks, has a
// non-public policy status, and has no public ACL grants.
func TestStateBucketIsDeployedAndPrivate(t *testing.T) {
	s := loadState(t)
	client := s3.NewFromConfig(s.cfg)

	if _, err := client.HeadBucket(s.ctx, &s3.HeadBucketInput{Bucket: &s.bucket}); err != nil {
		t.Fatalf("state bucket %q is not deployed or not readable: %v", s.bucket, err)
	}

	access, err := client.GetPublicAccessBlock(s.ctx, &s3.GetPublicAccessBlockInput{Bucket: &s.bucket})
	if err != nil {
		t.Fatalf("get public access block for %q: %v", s.bucket, err)
	}
	if access.PublicAccessBlockConfiguration == nil {
		t.Fatalf("state bucket %q has no public access block", s.bucket)
	}
	pab := access.PublicAccessBlockConfiguration
	for _, block := range []struct {
		name  string
		value *bool
	}{
		{"BlockPublicAcls", pab.BlockPublicAcls},
		{"IgnorePublicAcls", pab.IgnorePublicAcls},
		{"BlockPublicPolicy", pab.BlockPublicPolicy},
		{"RestrictPublicBuckets", pab.RestrictPublicBuckets},
	} {
		if block.value == nil || !*block.value {
			t.Errorf("state bucket %q: %s is not true", s.bucket, block.name)
		}
	}

	policyStatus, err := client.GetBucketPolicyStatus(s.ctx, &s3.GetBucketPolicyStatusInput{Bucket: &s.bucket})
	if err != nil {
		t.Fatalf("get policy status for %q: %v", s.bucket, err)
	}
	if policyStatus.PolicyStatus == nil || policyStatus.PolicyStatus.IsPublic == nil {
		t.Fatalf("state bucket %q has no policy status", s.bucket)
	}
	if *policyStatus.PolicyStatus.IsPublic {
		t.Errorf("state bucket %q has a public bucket policy", s.bucket)
	}

	acl, err := client.GetBucketAcl(s.ctx, &s3.GetBucketAclInput{Bucket: &s.bucket})
	if err != nil {
		t.Fatalf("get ACL for %q: %v", s.bucket, err)
	}
	for _, grant := range acl.Grants {
		if grant.Grantee == nil || grant.Grantee.URI == nil {
			continue
		}
		if isPublicGroup(*grant.Grantee.URI) {
			t.Errorf("state bucket %q grants access to public group %s", s.bucket, *grant.Grantee.URI)
		}
	}

	t.Logf("state bucket %s exists and is not public", s.bucket)
}

// TestStateBucketHasNoWebsiteConfiguration is the negative counterpart to the
// public-listing checks: a bucket with website hosting configured can serve
// objects without an S3 ListBucket grant.
func TestStateBucketHasNoWebsiteConfiguration(t *testing.T) {
	s := loadState(t)
	client := s3.NewFromConfig(s.cfg)

	_, err := client.GetBucketWebsite(s.ctx, &s3.GetBucketWebsiteInput{Bucket: &s.bucket})
	if err == nil {
		t.Fatalf("state bucket %q has website hosting configured", s.bucket)
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NoSuchWebsiteConfiguration" {
		t.Fatalf("GetBucketWebsite(%q) = %v, want NoSuchWebsiteConfiguration", s.bucket, err)
	}
}

// TestStateBucketIsNotPublicOverHTTPS is the external proof: an anonymous GET
// of the bucket root must not list objects. A 200 here means the bucket is
// public regardless of what the policy-status API says.
func TestStateBucketIsNotPublicOverHTTPS(t *testing.T) {
	s := loadState(t)
	endpoint := bucketURL(s.bucket, s.region)

	resp := fetch(t, s.ctx, endpoint)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("anonymous GET %s = %d, want 403 for a private bucket", endpoint, resp.StatusCode)
	}
	t.Logf("anonymous GET %s = 403; bucket is not publicly listable", endpoint)
}

// TestPublicBucketFetchSucceeds is the positive control for the fetch itself.
func TestPublicBucketFetchSucceeds(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	resp := fetch(t, ctx, publicBucketURL)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", publicBucketURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "<ListBucketResult") {
		t.Fatalf("GET %s returned 200 but not an S3 listing: %q", publicBucketURL, body)
	}
	t.Logf("anonymous GET %s = 200 listing; fetch positive control works", publicBucketURL)
}

func fetch(t *testing.T, ctx context.Context, url string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request for %s: %v", url, err)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func stateBucket(ctx context.Context, cfg aws.Config) (string, error) {
	if bucket := os.Getenv("STATE_BUCKET"); bucket != "" {
		return bucket, nil
	}
	account, err := accountID(ctx, cfg)
	if err != nil {
		return "", err
	}
	return bucketPrefix + account, nil
}

func bucketURL(bucket, region string) string {
	if region == "us-east-1" {
		return fmt.Sprintf("https://%s.s3.amazonaws.com/", bucket)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/", bucket, region)
}

func awsRegion() string {
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
