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
