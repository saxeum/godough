locals {
  charts = "${path.module}/../../../deploy/charts"

  # The charts' values live in values/*.yaml, not in this file: a deployment is
  # configuration that a reviewer should be able to read without parsing HCL, and
  # a HCL map that mirrors the chart's schema is the same thing written twice.
  #
  # Four values are templated rather than written down, because they are not
  # configuration: the image digest is a decision made at deploy time, and the
  # certificate, log bucket and WAF ACL only exist once this stack has created
  # them.
  api_values = templatefile("${path.module}/values/api.yaml", {
    image_repository = "${var.image_registry}/${var.project}-api"
    image_digest     = var.api_digest
    trusted_origins  = var.trusted_origins
    ingress_host     = var.domain
    certificate_arn  = aws_acm_certificate_validation.edge.certificate_arn
    log_bucket       = aws_s3_bucket.alb_logs.id
    waf_acl_arn      = aws_wafv2_web_acl.edge.arn
  })

  # -web, not -ui: the repositories are named for the surfaces (#103), and the
  # first apply of this deployed a digest that resolved against a repository that
  # does not exist, which reads as "not found" rather than "wrong name".
  ui_values = templatefile("${path.module}/values/ui.yaml", {
    image_repository = "${var.image_registry}/${var.project}-web"
    image_digest     = var.ui_digest
    ingress_host     = var.domain
    certificate_arn  = aws_acm_certificate_validation.edge.certificate_arn
    log_bucket       = aws_s3_bucket.alb_logs.id
    waf_acl_arn      = aws_wafv2_web_acl.edge.arn
  })
}

# ---- Argo CD ---------------------------------------------------------------
#
# The bootstrap for #127. Terraform installs it once; after the handover this file
# no longer owns the two workloads and Argo reconciles them from git.
#
# Installed here rather than by an Application: an Argo that manages its own
# installation cannot be debugged by the thing it manages when it breaks.
resource "helm_release" "argocd" {
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  version          = "10.20.0"
  namespace        = "argocd"
  create_namespace = true

  values = [file("${path.module}/values/argocd.yaml")]

  # Applied means running: its CRDs and controller have to be ready before
  # anything is pointed at them.
  wait    = true
  timeout = 900

  depends_on = [helm_release.lbc]
}

resource "helm_release" "api" {
  name      = "godough-api"
  chart     = "${local.charts}/api"
  namespace = var.namespace
  # Created here rather than by a second provider for two workloads.
  create_namespace = true

  values = [local.api_values]

  # Applied means running, not merely applied. Without this a rollout that
  # cannot start (a missing Secret, an unpullable digest) reports success.
  wait    = true
  timeout = 300

  # The policy, not just the bucket: the listener is told to write before the
  # bucket would accept it otherwise, and the first logs would be a silent 403.
  depends_on = [helm_release.lbc, aws_acm_certificate_validation.edge, aws_s3_bucket_policy.alb_logs]
}

resource "helm_release" "ui" {
  name             = "godough-ui"
  chart            = "${local.charts}/ui"
  namespace        = var.namespace
  create_namespace = false

  values = [local.ui_values]

  wait    = true
  timeout = 300

  depends_on = [helm_release.lbc, aws_acm_certificate_validation.edge, helm_release.api, aws_s3_bucket_policy.alb_logs]
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

# The zone is in Cloudflare, so the validation record goes there. The token comes
# from CLOUDFLARE_API_TOKEN in the environment and needs Zone:Read and Zone:DNS:
# Edit on this one zone — nothing account-wide.
data "cloudflare_zone" "edge" {
  filter = {
    name = var.cloudflare_zone
  }
}

# DNS only. ACM fetches this name itself and a proxied record would answer with
# Cloudflare's edge addresses instead of the value it is looking for.
resource "cloudflare_dns_record" "certificate_validation" {
  for_each = {
    for option in aws_acm_certificate.edge.domain_validation_options : option.domain_name => option
  }

  zone_id = data.cloudflare_zone.edge.zone_id
  name    = trimsuffix(each.value.resource_record_name, ".")
  type    = each.value.resource_record_type
  content = trimsuffix(each.value.resource_record_value, ".")
  ttl     = 300
  proxied = false
}

# Waits for the certificate to be issued, so anything referencing its ARN is
# planning against a certificate that exists rather than one that is pending.
resource "aws_acm_certificate_validation" "edge" {
  certificate_arn = aws_acm_certificate.edge.arn

  validation_record_fqdns = [for record in cloudflare_dns_record.certificate_validation : record.name]
}

# Grey (DNS only), on purpose: clients reach the ALB and see its ACM certificate,
# so there is no second TLS hop to configure and nothing to trust at the edge. The
# proxy would have to be full-strict to be worth anything, and that belongs with
# CloudFront (#126's later half), not here.
#
# The name comes from the Ingress, which creates the load balancer: Terraform
# cannot know the ALB's DNS name before that exists, so this is one value to set
# once, from `kubectl -n godough get ingress godough-api -o jsonpath` .
resource "cloudflare_dns_record" "edge" {
  count = var.alb_hostname == "" ? 0 : 1

  zone_id = data.cloudflare_zone.edge.zone_id
  name    = var.domain
  type    = "CNAME"
  content = var.alb_hostname
  ttl     = 300
  proxied = false
}

# ---- ALB access logs -------------------------------------------------------
#
# The ALB keeps no record of what hit it, so a flood takes the site down and leaves
# nothing behind to say who sent it, from where, or how much (#171). Request volume
# is the input a rate limit or a WAF rule needs, and there is no other place to get
# it.
#
# SSE-S3, not the customer KMS key the state bucket uses: ALB access logs do not
# support SSE-KMS with a customer-managed key. This is the one bucket in the account
# that is not on the KMS pattern, and the exception is deliberate.

resource "aws_s3_bucket" "alb_logs" {
  bucket = "${var.project}-alb-logs-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket_public_access_block" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# ObjectWriter rather than BucketOwnerEnforced: the delivery service writes the
# object and asks for bucket-owner-full-control, which only does anything while
# ACLs are enabled. Public access stays blocked either way, and the policy below is
# the only grant.
resource "aws_s3_bucket_ownership_controls" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    object_ownership = "ObjectWriter"
  }
}

# `iac` flags this bucket as AWS-0132: SSE-S3 where the rule wants a customer
# managed key. The restriction trivy cites ("SSE-KMS is not supported for S3
# server access logging destination buckets") is about a different feature — S3
# server access logs — and whether ALB log delivery accepts a CMK is **not
# established here**. Suppressed so the logs can ship; the experiment that would
# remove it is #176, and the reason to expect it might work is that the delivery
# principal is named explicitly in the policy below and could be granted the key.
#trivy:ignore:AWS-0132
resource "aws_s3_bucket_server_side_encryption_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Thirty days, hardcoded rather than a variable: at this traffic it is a few
# megabytes, and a knob with one value is a constant someone else has to read.
resource "aws_s3_bucket_lifecycle_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    id     = "expire-logs"
    status = "Enabled"

    filter {}

    expiration {
      days = 30
    }

    abort_incomplete_multipart_upload {
      days_after_initiation = 1
    }
  }
}

data "aws_iam_policy_document" "alb_logs" {
  # The service principal, not the region's log-delivery account id: the id is a
  # number that goes stale in the next region, and this is the form AWS documents
  # for load balancer access logs.
  statement {
    sid    = "ELBAccessLogs"
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["logdelivery.elasticloadbalancing.amazonaws.com"]
    }

    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.alb_logs.arn}/godough/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]

    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
  }

  statement {
    sid    = "DenyInsecureTransport"
    effect = "Deny"

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    actions   = ["s3:*"]
    resources = [aws_s3_bucket.alb_logs.arn, "${aws_s3_bucket.alb_logs.arn}/*"]

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "alb_logs" {
  bucket     = aws_s3_bucket.alb_logs.id
  policy     = data.aws_iam_policy_document.alb_logs.json
  depends_on = [aws_s3_bucket_public_access_block.alb_logs]
}

# ---- WAF -------------------------------------------------------------------
#
# The first control in this stack that can stop a flood before it reaches the
# database. One replica and a five-connection pool means a modest burst from one
# source is enough to make the site unavailable, and nothing upstream was counting
# (#174).
#
# COUNT MODE. Every rule is present and evaluating, and none of them blocks yet.
# The point is to read what would have been blocked — the access logs from #171 are
# the input — and then move the rules that actually fire to block. A WAF switched
# straight into block mode on a working site is how a launch takes itself down at
# the first false positive.

resource "aws_wafv2_web_acl" "edge" {
  name        = "${var.project}-edge"
  description = "Rules in front of the godough ALB. Count mode until the logs say otherwise."
  scope       = "REGIONAL"

  default_action {
    allow {}
  }

  rule {
    name     = "aws-common"
    priority = 1

    override_action {
      count {}
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${var.project}-common"
      sampled_requests_enabled   = true
    }
  }

  # The number is measured, not chosen: the access logs from #171 showed 49
  # requests in 36 minutes across 26 addresses, with a busiest window of **6
  # requests per address per five minutes**. 100 is sixteen times that, which
  # leaves room for a browser loading the page a few times and still stops a
  # sustained flood from one address. The sample is small — one hour of a staging
  # site — so this is a floor with a stated margin, not a finding; revisit it when
  # the logs have a week in them.
  #
  # Scoped to /api/ because that is the only path that costs a database round
  # trip before it can be refused; the SPA's assets are cached and cheap.
  rule {
    name     = "api-rate"
    priority = 2

    action {
      count {}
    }

    statement {
      rate_based_statement {
        limit              = 100
        aggregate_key_type = "IP"

        scope_down_statement {
          byte_match_statement {
            search_string         = "/api/"
            positional_constraint = "STARTS_WITH"

            field_to_match {
              uri_path {}
            }

            text_transformation {
              priority = 0
              type     = "NONE"
            }
          }
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${var.project}-api-rate"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.project}-edge"
    sampled_requests_enabled   = true
  }
}
