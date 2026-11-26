variable "aws_region" {
  description = "AWS region for the cluster."
  type        = string
  default     = "ap-southeast-2"
}

variable "project" {
  description = "Project prefix for names and the state bucket."
  type        = string
  default     = "godough"
}

variable "cluster_version" {
  description = "Kubernetes version. The newest standard-support EKS version ages out last."
  type        = string
  default     = "1.37"
}

variable "node_instance_types" {
  description = "Node instance types. x86_64, because the published images are amd64-only: t4g.small is cheaper and would need a multi-arch build first (#134)."
  type        = list(string)
  default     = ["t3.small"]
}

variable "node_desired_size" {
  description = "Nodes to run. One, because staging serves nobody."
  type        = number
  default     = 1
}

variable "node_min_size" {
  description = "Nodes the group will not go below."
  type        = number
  default     = 1
}

variable "node_max_size" {
  description = "Nodes the group will not exceed. Two covers a rolling replacement without a scheduling gap."
  type        = number
  default     = 2
}
