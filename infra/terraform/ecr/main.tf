data "aws_caller_identity" "current" {}

locals {
  api_repository = "${var.project}-api"
  web_repository = "${var.project}-web"
  state_bucket   = "${var.project}-terraform-state-${data.aws_caller_identity.current.account_id}"
  github_owner   = split("/", var.github_repository)[0]
  github_repo    = split("/", var.github_repository)[1]
  github_subject = "repo:${local.github_owner}@${var.github_owner_id}/${local.github_repo}@${var.github_repository_id}:ref:refs/heads/${var.github_branch}"
}

resource "aws_ecr_repository" "api" {
  name                 = local.api_repository
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }
}

resource "aws_ecr_repository" "web" {
  name                 = local.web_repository
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }
}

resource "aws_ecr_lifecycle_policy" "api" {
  repository = aws_ecr_repository.api.name
  policy     = data.aws_ecr_lifecycle_policy_document.default.json
}

resource "aws_ecr_lifecycle_policy" "web" {
  repository = aws_ecr_repository.web.name
  policy     = data.aws_ecr_lifecycle_policy_document.default.json
}

data "aws_ecr_lifecycle_policy_document" "default" {
  rule {
    priority    = 1
    description = "Expire untagged images after 7 days"

    selection {
      tag_status   = "untagged"
      count_type   = "sinceImagePushed"
      count_unit   = "days"
      count_number = 7
    }

    action {
      type = "expire"
    }
  }

  rule {
    priority    = 2
    description = "Keep the most recent 20 tagged images"

    selection {
      tag_status       = "tagged"
      tag_pattern_list = ["*"]
      count_type       = "imageCountMoreThan"
      count_number     = 20
    }

    action {
      type = "expire"
    }
  }
}

# GitHub Actions OIDC. No static AWS keys in the repository or the workflow.
resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
  thumbprint_list = [
    # Old GitHub/DigiCert chain, retained for overlap.
    "6938fd4d98bab03faadb97b34396831e3780aea1",
    "1c58a3a8518e8759bf075b76b750d4f2df264fcd",
    # Current GitHub chain: Let's Encrypt YR1 / Root YR.
    "cbcfd09ce3cd22d99d94e58f92e9898fbee51a5e",
    "ab9d0263244dd0326eb67015705a667e79cfe998",
  ]
}

data "aws_iam_policy_document" "github_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = [local.github_subject]
    }
  }
}

resource "aws_iam_role" "github_publish" {
  name               = "${var.project}-github-publish"
  assume_role_policy = data.aws_iam_policy_document.github_assume.json
}

data "aws_iam_policy_document" "github_publish" {
  statement {
    sid       = "EcrLogin"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  statement {
    sid = "PushAndPullImages"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:CompleteLayerUpload",
      "ecr:GetDownloadUrlForLayer",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
    ]
    resources = [
      aws_ecr_repository.api.arn,
      aws_ecr_repository.web.arn,
    ]
  }
}

data "aws_iam_policy_document" "github_aws_test" {
  statement {
    sid = "ReadStateBucketSecurityConfig"
    actions = [
      "s3:GetBucketAcl",
      "s3:GetBucketLocation",
      "s3:GetBucketPolicyStatus",
      "s3:GetBucketPublicAccessBlock",
      "s3:GetBucketWebsite",
    ]
    resources = ["arn:aws:s3:::${local.state_bucket}"]
  }

  statement {
    sid = "ReadEcrMetadata"
    actions = [
      "ecr:DescribeRepositories",
      "ecr:GetLifecyclePolicy",
      "ecr:GetRepositoryPolicy",
    ]
    resources = [
      aws_ecr_repository.api.arn,
      aws_ecr_repository.web.arn,
    ]
  }

  statement {
    sid       = "ReadOwnRole"
    actions   = ["iam:GetRole"]
    resources = [aws_iam_role.github_publish.arn]
  }
}

resource "aws_iam_role_policy" "github_publish" {
  name   = "${var.project}-github-publish"
  role   = aws_iam_role.github_publish.id
  policy = data.aws_iam_policy_document.github_publish.json
}

resource "aws_iam_role_policy" "github_aws_test" {
  name   = "${var.project}-github-aws-test"
  role   = aws_iam_role.github_publish.id
  policy = data.aws_iam_policy_document.github_aws_test.json
}
