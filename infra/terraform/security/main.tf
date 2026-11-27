# Account-level detection.
#
# GuardDuty is the one control in this repository that notices something happening
# rather than preventing it: a credential used from somewhere new, an instance
# mining, the EKS role assumed from a network this account does not use.
# Everything else in docs/security-review.md stops things; this is the alarm
# (#172).
#
# Foundational only. Enabling the detector turns on its own data sources — VPC
# flow logs, DNS and CloudTrail management events — and nothing else. The
# per-feature protections (S3 data events, EKS audit logs, RDS login events,
# malware scanning) each carry their own bill and their own decision, so they are
# a later ticket rather than a silent default here.

resource "aws_guardduty_detector" "this" {
  enable = true
}

# ---- CloudTrail ------------------------------------------------------------
#
# The trail existed and was logging, but every delivery since 2026-10-08 failed
# with NoSuchBucket: the S3 bucket it was configured with had been deleted, and
# the trail went on reporting `IsLogging: true` while the account's only long-term
# audit record was empty (#173).
#
# Note what "no longer delivered" does not mean: CloudTrail event history still
# answers `lookup-events` for 90 days. That is the query window, not an archive —
# the archive is the bucket, and it had been losing entries for seven weeks before
# anyone looked.

data "aws_caller_identity" "current" {}

resource "aws_s3_bucket" "cloudtrail" {
  bucket = "${var.project}-cloudtrail-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket_public_access_block" "cloudtrail" {
  bucket = aws_s3_bucket.cloudtrail.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Versioning, because an audit object that can be overwritten in place is not
# evidence. A delivery that lands on the same key twice keeps both copies.
resource "aws_s3_bucket_versioning" "cloudtrail" {
  bucket = aws_s3_bucket.cloudtrail.id

  versioning_configuration {
    status = "Enabled"
  }
}

# SSE-S3 rather than a customer key, for the same reason as the ALB log bucket:
# the trail is not configured with a KMS key, and switching it to one is a
# separate decision about who can read the account's audit history. #176 is where
# that argument is being had for the other bucket.
resource "aws_s3_bucket_server_side_encryption_configuration" "cloudtrail" {
  bucket = aws_s3_bucket.cloudtrail.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# A year, not thirty days: the point of this bucket is the thing that answers
# "what happened in June", and a quarter of retention is a quarter of an audit
# trail.
resource "aws_s3_bucket_lifecycle_configuration" "cloudtrail" {
  bucket = aws_s3_bucket.cloudtrail.id

  rule {
    id     = "expire-logs"
    status = "Enabled"

    filter {}

    expiration {
      days = 365
    }

    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }
}

data "aws_iam_policy_document" "cloudtrail" {
  statement {
    sid    = "AWSCloudTrailAclCheck"
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }

    actions   = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.cloudtrail.arn]
  }

  statement {
    sid    = "AWSCloudTrailWrite"
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }

    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.cloudtrail.arn}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]

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
    resources = [aws_s3_bucket.cloudtrail.arn, "${aws_s3_bucket.cloudtrail.arn}/*"]

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "cloudtrail" {
  bucket     = aws_s3_bucket.cloudtrail.id
  policy     = data.aws_iam_policy_document.cloudtrail.json
  depends_on = [aws_s3_bucket_public_access_block.cloudtrail]
}

# Imported from the account rather than recreated: the trail has two years of
# configuration and custom event selectors that are not modelled here, and
# `terraform import aws_cloudtrail.management management-events` brings it under
# management without touching them. This block then changes exactly two things —
# the bucket it writes to, and the validation flag.
resource "aws_cloudtrail" "management" {
  name                          = "management-events"
  s3_bucket_name                = aws_s3_bucket.cloudtrail.id
  include_global_service_events = true
  is_multi_region_trail         = true
  enable_log_file_validation    = true

  # Modelled because the import found it, not because this change wants it. The
  # trail has had a custom "Management events" selector since it was created in
  # the console; leaving it out of the configuration makes the first apply delete
  # it, which would stop the trail recording anything — a repair that becomes a
  # second outage. It is written here exactly as the account has it.
  advanced_event_selector {
    name = "Management events selector"

    field_selector {
      field  = "eventCategory"
      equals = ["Management"]
    }
  }

  # The policy has to allow the write before the trail is repointed at the
  # bucket, or the first deliveries are the same silent NoSuchBucket with a
  # different reason in the status.
  depends_on = [aws_s3_bucket_policy.cloudtrail]
}
