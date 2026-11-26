package awstest

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

const (
	apiRepository = "godough-api"
	webRepository = "godough-web"
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
