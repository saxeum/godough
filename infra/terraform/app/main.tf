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

  values = [file("${path.module}/../../../deploy/argocd/install/values.yaml")]

  # Applied means running: its CRDs and controller have to be ready before
  # anything is pointed at them.
  wait    = true
  timeout = 900

  depends_on = [helm_release.lbc]
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

  # The anonymous half, keyed on the address the ALB saw.
  #
  # The number comes from the access logs (#171) over the first 4.5 hours: 235
  # requests from 99 addresses, a busiest legitimate window of **9 per address per
  # five minutes**, and 32 when a test deliberately burst one. 100 is ten times the
  # legitimate peak, which leaves room for a browser reloading the page and still
  # cuts off a sustained flood. It is a floor with a stated margin, not a
  # measurement of what a busy day looks like.
  #
  # Scoped to /api/ because that is the only path that costs a database round trip
  # before it can be refused; the SPA's assets are cached and cheap.
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

  # The signed-in half, aggregated on the session cookie rather than the address.
  #
  # A per-address rule cannot see an account at all, and one account behind a
  # rotating set of addresses is one client to this rule and many to the one above.
  # The bucket here is per session cookie value.
  #
  # The scope-down is not decoration: a custom-key rule aggregates requests *missing*
  # the key into one bucket, so without "the cookie is present" every anonymous
  # visitor shares a bucket and, once this rule blocks, a burst of sign-ins locks all
  # of them out together. Anonymous traffic stays with api-rate, which keys on
  # address.
  #
  # Measured, because the first attempt at this was wrong: a size_constraint on the
  # cookie value never matched, and a byte_match on the cookie *key* does. Both were
  # tried against a probe path before this was written.
  #
  # What this does not do: bound a burst. The counter lags the traffic — 12 requests
  # passed at a limit of 10 before blocking began — so the opening seconds of a flood
  # get through. It stops sustained abuse from one session; storage is bounded by a
  # per-user quota, not by a rate.
  #
  # 300 per five minutes is 1 request/second sustained. A page load is about five
  # requests and the app does not poll, so this is a policy number with room to
  # spare — the traffic so far has almost no signed-in writes to measure.
  rule {
    name     = "session-rate"
    priority = 3

    action {
      count {}
    }

    statement {
      rate_based_statement {
        limit              = 300
        aggregate_key_type = "CUSTOM_KEYS"

        custom_key {
          cookie {
            name = "session"

            text_transformation {
              priority = 0
              type     = "NONE"
            }
          }
        }

        scope_down_statement {
          byte_match_statement {
            search_string         = "session"
            positional_constraint = "EXACTLY"

            field_to_match {
              cookies {
                match_scope       = "KEY"
                oversize_handling = "MATCH"

                match_pattern {
                  included_cookies = ["session"]
                }
              }
            }

            text_transformation {
              priority = 0
              type     = "LOWERCASE"
            }
          }
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${var.project}-session-rate"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.project}-edge"
    sampled_requests_enabled   = true
  }
}

# ---- WAF logs --------------------------------------------------------------
#
# Metrics are not enough to run a WAF. `CountedRequests` says a rule matched; it does
# not say which one, and a count-mode rate rule publishes no metric at all (#187).
# Without the log, "count mode is observing, so we can decide what to block" is an
# assumption — and one that was demonstrated to be unobservable.
#
# The log group name must start with `aws-waf-logs-`: that prefix is how the service
# decides a group is an allowed destination, so it is part of the contract rather
# than a naming preference.

resource "aws_cloudwatch_log_group" "waf" {
  name = "aws-waf-logs-${var.project}-edge"

  # Thirty days: long enough to read a week of traffic and decide, short enough that
  # nobody has to think about the bill.
  retention_in_days = 30
}

data "aws_iam_policy_document" "waf_logs" {
  statement {
    sid    = "AllowWAFLogDelivery"
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["delivery.logs.amazonaws.com"]
    }

    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.waf.arn}:*"]

    # No aws:SourceAccount condition, deliberately and unlike the ALB log bucket.
    # WAF's log delivery does not assert that key, and a condition it cannot satisfy
    # denies the write silently: the block happens, the metric counts it, and nothing
    # appears in the group. The resource is already scoped to one log group in one
    # account, which is the guarantee the condition was there to add.
  }
}

resource "aws_cloudwatch_log_resource_policy" "waf_logs" {
  policy_name     = "${var.project}-waf-log-delivery"
  policy_document = data.aws_iam_policy_document.waf_logs.json
}

resource "aws_wafv2_web_acl_logging_configuration" "edge" {
  resource_arn            = aws_wafv2_web_acl.edge.arn
  log_destination_configs = [aws_cloudwatch_log_group.waf.arn]

  # The session cookie is a bearer token. A WAF log records the request headers, so
  # without this every rule match would put a usable credential in a log group that
  # a wider set of people can read than can read the database. Nothing in a decision
  # about which rule fired needs the token, and the rule's aggregation key is logged
  # by WAF separately as a hash-free key value only when it blocks.
  redacted_fields {
    single_header {
      name = "cookie"
    }
  }

  redacted_fields {
    single_header {
      name = "authorization"
    }
  }

  depends_on = [aws_cloudwatch_log_resource_policy.waf_logs]
}
