locals {
  charts = "${path.module}/../../../deploy/charts"

  api_values = {
    image = {
      repository = "${var.image_registry}/${var.project}-api"
      digest     = var.api_digest
    }
    trustedOrigins = var.trusted_origins
    # The Secrets are created by `make rds-app-password` and the sync that #125
    # decides on; the chart only names them.
    databaseSecret = { name = "godough-api", key = "DATABASE_URL" }
    caSecret       = { name = "godough-rds-ca", key = "global-bundle.pem" }
  }

  ui_values = {
    image = {
      # -web, not -ui: the repositories are named for the surfaces (#103), and the
      # first apply of this deployed a digest that resolved against a repository
      # that does not exist, which reads as "not found" rather than "wrong name".
      repository = "${var.image_registry}/${var.project}-web"
      digest     = var.ui_digest
    }
  }
}

resource "helm_release" "api" {
  name      = "godough-api"
  chart     = "${local.charts}/api"
  namespace = var.namespace
  # Created here rather than by a second provider for two workloads.
  create_namespace = true

  values = [yamlencode(local.api_values)]

  # Applied means running, not merely applied. Without this a rollout that
  # cannot start (a missing Secret, an unpullable digest) reports success.
  wait    = true
  timeout = 300
}

resource "helm_release" "ui" {
  name             = "godough-ui"
  chart            = "${local.charts}/ui"
  namespace        = var.namespace
  create_namespace = false

  values = [yamlencode(local.ui_values)]

  wait    = true
  timeout = 300

  depends_on = [helm_release.api]
}

# ---- AWS Load Balancer Controller ------------------------------------------
#
# The controller is what turns an Ingress into an ALB with pod IPs in the target
# groups. A hand-built aws_lb cannot do that: instance targets would route through
# the node's kube-proxy port and lose the pod's identity, which readiness and
# per-pod health checks both need.
#
# It lives in this stack because it needs the Helm provider, which needs the
# tunnel. The cluster stack stays pure AWS API and can be applied with the
# tunnel down.

data "aws_caller_identity" "current" {}

data "terraform_remote_state" "eks" {
  backend = "s3"

  config = {
    bucket = "${var.project}-terraform-state-${data.aws_caller_identity.current.account_id}"
    key    = "eks/terraform.tfstate"
    region = var.aws_region
  }
}

data "terraform_remote_state" "data" {
  backend = "s3"

  config = {
    bucket = "${var.project}-terraform-state-${data.aws_caller_identity.current.account_id}"
    key    = "data/terraform.tfstate"
    region = var.aws_region
  }
}

locals {
  cluster_name = data.terraform_remote_state.eks.outputs.cluster_name
  oidc_host    = replace(data.terraform_remote_state.eks.outputs.oidc_issuer, "https://", "")
  vpc_id       = data.terraform_remote_state.data.outputs.vpc_id
  lbc_sa       = "system:serviceaccount:kube-system:aws-load-balancer-controller"
}

data "aws_iam_policy_document" "lbc_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [data.terraform_remote_state.eks.outputs.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:sub"
      values   = [local.lbc_sa]
    }
  }
}

resource "aws_iam_role" "lbc" {
  name               = "${local.cluster_name}-lbc"
  assume_role_policy = data.aws_iam_policy_document.lbc_assume.json
}

# Vendored rather than referenced: the policy is what makes the controller able
# to create and delete load balancers, so it is reviewed like code. The file is
# upstream's docs/install/iam_policy.json at v3.7.0, the chart's version.
resource "aws_iam_policy" "lbc" {
  name   = "${local.cluster_name}-lbc"
  policy = file("${path.module}/policies/aws-load-balancer-controller.json")
}

resource "aws_iam_role_policy_attachment" "lbc" {
  role       = aws_iam_role.lbc.name
  policy_arn = aws_iam_policy.lbc.arn
}

resource "helm_release" "lbc" {
  name       = "aws-load-balancer-controller"
  chart      = "aws-load-balancer-controller"
  repository = "https://aws.github.io/eks-charts"
  version    = "3.7.0"
  namespace  = "kube-system"

  set = [
    { name = "clusterName", value = local.cluster_name },
    { name = "region", value = var.aws_region },
    { name = "vpcId", value = local.vpc_id },
    { name = "serviceAccount.create", value = "true" },
    { name = "serviceAccount.name", value = "aws-load-balancer-controller" },
    { name = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn", value = aws_iam_role.lbc.arn },
  ]

  wait    = true
  timeout = 300
}

# ---- the edge certificate --------------------------------------------------
#
# Created here so the name is recorded in one place, but it cannot be validated
# by Terraform: the validation CNAME lives in Cloudflare, and this configuration
# has no Cloudflare credentials. See README.md for the two records, and
# `terraform output certificate_validation_record` for the exact values.
resource "aws_acm_certificate" "edge" {
  domain_name       = var.domain
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}
