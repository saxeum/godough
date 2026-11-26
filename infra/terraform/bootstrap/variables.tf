variable "aws_region" {
  description = "AWS region for the state bucket."
  type        = string
  default     = "ap-southeast-2"
}

variable "project" {
  description = "Project prefix for the state bucket name."
  type        = string
  default     = "godough"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,30}$", var.project))
    error_message = "project must be lowercase letters, digits and hyphens, starting with a letter."
  }
}
