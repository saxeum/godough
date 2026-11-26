output "state_bucket" {
  description = "S3 bucket for Terraform state."
  value       = aws_s3_bucket.state.bucket
}

output "state_bucket_arn" {
  description = "ARN of the Terraform state bucket."
  value       = aws_s3_bucket.state.arn
}

output "state_region" {
  description = "Region of the Terraform state bucket."
  value       = var.aws_region
}
