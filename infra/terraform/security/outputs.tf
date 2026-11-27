output "detector_id" {
  description = "The GuardDuty detector, for the console and for exercises."
  value       = aws_guardduty_detector.this.id
}

output "trail_bucket" {
  description = "Bucket the management-events trail writes to."
  value       = aws_s3_bucket.cloudtrail.id
}
