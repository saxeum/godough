variable "aws_region" {
  description = "Region the account-level controls apply to. GuardDuty is regional."
  type        = string
  default     = "ap-southeast-2"
}

variable "project" {
  description = "Project prefix, used for names and the state bucket."
  type        = string
  default     = "godough"
}
