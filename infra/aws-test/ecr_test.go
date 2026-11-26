package awstest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/smithy-go"
)

const (
	apiRepository = "godough-api"
	webRepository = "godough-web"

	// GitHub's OIDC subject now carries the immutable owner and repository
	// ids. Keep this exact: a name-only subject would let a renamed repo match.
	githubSubject = "repo:saxeum@343752441/godough@1472378230:ref:refs/heads/master"
)

// TestECRRepositoriesAreImmutableAndScanned is the #103 check: both images
// exist, their tags cannot be overwritten, and every push is scanned.
func TestECRRepositoriesAreImmutableAndScanned(t *testing.T) {
	ctx := t.Context()
	client := ecr.NewFromConfig(loadAWS(t))

	out, err := client.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{apiRepository, webRepository},
	})
	if err != nil {
		t.Fatalf("describe ECR repositories: %v", err)
	}
	if len(out.Repositories) != 2 {
		t.Fatalf("got %d repositories, want 2", len(out.Repositories))
	}

	for _, repo := range out.Repositories {
		name := *repo.RepositoryName
		if got := string(repo.ImageTagMutability); got != "IMMUTABLE" {
			t.Errorf("%s: tag mutability = %s, want IMMUTABLE", name, got)
		}
		if repo.ImageScanningConfiguration == nil || !repo.ImageScanningConfiguration.ScanOnPush {
			t.Errorf("%s: scan on push is not enabled", name)
		}
		if repo.EncryptionConfiguration == nil || string(repo.EncryptionConfiguration.EncryptionType) != "AES256" {
			t.Errorf("%s: encryption is not AES256", name)
		}
	}
}

// TestECRRepositoriesHaveNoRepositoryPolicy is the negative access check: a
// repository policy is how an ECR repository becomes cross-account readable.
func TestECRRepositoriesHaveNoRepositoryPolicy(t *testing.T) {
	ctx := t.Context()
	client := ecr.NewFromConfig(loadAWS(t))

	for _, repository := range []string{apiRepository, webRepository} {
		_, err := client.GetRepositoryPolicy(ctx, &ecr.GetRepositoryPolicyInput{RepositoryName: &repository})
		if err == nil {
			t.Fatalf("%s has a repository policy", repository)
		}
		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "RepositoryPolicyNotFoundException" {
			t.Fatalf("%s: GetRepositoryPolicy = %v, want RepositoryPolicyNotFoundException", repository, err)
		}
	}
}

// TestGitHubPublishRoleTrustIsNarrow is the access-control check: the publish
// role must only be assumable by this repository on master, through the GitHub
// OIDC provider.
func TestGitHubPublishRoleTrustIsNarrow(t *testing.T) {
	ctx := t.Context()
	client := iam.NewFromConfig(loadAWS(t))

	out, err := client.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String("godough-github-publish")})
	if err != nil {
		t.Fatalf("get publish role: %v", err)
	}
	if out.Role.AssumeRolePolicyDocument == nil {
		t.Fatal("publish role has no trust policy")
	}
	decoded, err := url.QueryUnescape(*out.Role.AssumeRolePolicyDocument)
	if err != nil {
		t.Fatalf("decode trust policy: %v", err)
	}

	var policy struct {
		Statement []struct {
			Action    any                       `json:"Action"`
			Principal map[string]any            `json:"Principal"`
			Condition map[string]map[string]any `json:"Condition"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(decoded), &policy); err != nil {
		t.Fatalf("parse trust policy: %v", err)
	}
	if len(policy.Statement) != 1 {
		t.Fatalf("got %d trust statements, want 1", len(policy.Statement))
	}

	statement := policy.Statement[0]
	if !strings.Contains(fmt.Sprint(statement.Action), "sts:AssumeRoleWithWebIdentity") {
		t.Errorf("trust action = %v, want AssumeRoleWithWebIdentity", statement.Action)
	}
	if !strings.Contains(fmt.Sprint(statement.Principal["Federated"]), "oidc-provider/token.actions.githubusercontent.com") {
		t.Errorf("trust principal = %v, want the GitHub OIDC provider", statement.Principal["Federated"])
	}
	aud := conditionValues(statement.Condition["StringEquals"]["token.actions.githubusercontent.com:aud"])
	if len(aud) != 1 || aud[0] != "sts.amazonaws.com" {
		t.Errorf("audience condition = %v, want sts.amazonaws.com", aud)
	}
	sub := conditionValues(statement.Condition["StringLike"]["token.actions.githubusercontent.com:sub"])
	if len(sub) != 1 || sub[0] != githubSubject {
		t.Errorf("subject condition = %v, want only %s", sub, githubSubject)
	}
}

func conditionValues(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case []string:
		return v
	default:
		return nil
	}
}

// TestECRLifecyclePolicies pins the rules #103 promises: untagged images expire
// after seven days, and the last twenty tagged images are kept.
func TestECRLifecyclePolicies(t *testing.T) {
	ctx := t.Context()
	client := ecr.NewFromConfig(loadAWS(t))

	for _, repository := range []string{apiRepository, webRepository} {
		out, err := client.GetLifecyclePolicy(ctx, &ecr.GetLifecyclePolicyInput{RepositoryName: &repository})
		if err != nil {
			t.Fatalf("%s: get lifecycle policy: %v", repository, err)
		}

		var policy struct {
			Rules []struct {
				Selection struct {
					TagStatus   string `json:"tagStatus"`
					CountType   string `json:"countType"`
					CountNumber int    `json:"countNumber"`
				} `json:"selection"`
			} `json:"rules"`
		}
		if err := json.Unmarshal([]byte(*out.LifecyclePolicyText), &policy); err != nil {
			t.Fatalf("%s: parse lifecycle policy: %v", repository, err)
		}

		var untagged, tagged bool
		for _, rule := range policy.Rules {
			switch {
			case rule.Selection.TagStatus == "untagged" && rule.Selection.CountType == "sinceImagePushed" && rule.Selection.CountNumber == 7:
				untagged = true
			case rule.Selection.TagStatus == "tagged" && rule.Selection.CountType == "imageCountMoreThan" && rule.Selection.CountNumber == 20:
				tagged = true
			}
		}
		if !untagged || !tagged {
			t.Errorf("%s: lifecycle policy missing rules; untagged=%v tagged=%v", repository, untagged, tagged)
		}
	}
}
