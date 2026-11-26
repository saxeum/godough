output "repository_urls" {
  description = "ECR repository URLs keyed by service."
  value = {
    api = aws_ecr_repository.api.repository_url
    web = aws_ecr_repository.web.repository_url
  }
}

output "registry" {
  description = "ECR registry host for both images."
  value       = split("/", aws_ecr_repository.api.repository_url)[0]
}

output "github_actions_role_arn" {
  description = "Role ARN for GitHub Actions; set it as the AWS_ROLE_ARN repository variable."
  value       = aws_iam_role.github_publish.arn
}
