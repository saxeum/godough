variable "aws_region" {
  description = "AWS region for the VPC and RDS instance."
  type        = string
  default     = "ap-southeast-2"
}

variable "project" {
  description = "Project prefix for names and tags."
  type        = string
  default     = "godough"
}

variable "vpc_cidr" {
  description = "CIDR for the staging VPC."
  type        = string
  default     = "10.42.0.0/16"
}

variable "db_instance_class" {
  description = "RDS instance class. Staging starts at the smallest Graviton size."
  type        = string
  default     = "db.t4g.micro"
}

variable "db_engine_version" {
  description = "RDS Postgres engine version."
  type        = string
  default     = "17.12"
}

variable "db_ca_cert_identifier" {
  description = "RDS CA identifier used by verify-full clients."
  type        = string
  default     = "rds-ca-rsa4096-g1"
}
