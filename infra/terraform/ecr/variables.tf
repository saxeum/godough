variable "aws_region" {
  description = "AWS region for the ECR repositories and the CI role."
  type        = string
  default     = "ap-southeast-2"
}

variable "project" {
  description = "Project prefix for the repository names."
  type        = string
  default     = "godough"
}

variable "github_repository" {
  description = "GitHub repository allowed to assume the publish role, owner/name."
  type        = string
  default     = "saxeum/godough"
}

variable "github_branch" {
  description = "Branch allowed to assume the publish role."
  type        = string
  default     = "master"
}

variable "github_owner_id" {
  description = "Immutable GitHub owner id used in the OIDC subject."
  type        = string
  default     = "343752441"
}

variable "github_repository_id" {
  description = "Immutable GitHub repository id used in the OIDC subject."
  type        = string
  default     = "1472378230"
}
