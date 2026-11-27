variable "aws_region" {
  description = "AWS region the registry and cluster live in."
  type        = string
  default     = "ap-southeast-2"
}

variable "project" {
  description = "Project prefix, used for names and the state bucket."
  type        = string
  default     = "godough"
}

variable "kubeconfig" {
  description = "Kubeconfig written by make eks-kubeconfig. It dials the tunnel, so `make eks-tunnel` must be running."
  type        = string
  default     = "~/.kube/config"
}

variable "kube_context" {
  description = "Context to use in that kubeconfig."
  type        = string
  default     = "godough-staging"
}

variable "namespace" {
  description = "Namespace both releases go into."
  type        = string
  default     = "godough"
}

variable "image_registry" {
  description = "ECR registry host, from the ecr stack's `registry` output."
  type        = string
}

# No defaults on purpose. A tag can move between review and rollout; a digest
# cannot, and a required variable is the only way to be sure the digest was a
# decision rather than whatever was lying around.
variable "api_digest" {
  description = "Digest of the api image to deploy, sha256:... from ECR."
  type        = string
}

variable "ui_digest" {
  description = "Digest of the ui image to deploy, sha256:... from ECR."
  type        = string
}

variable "trusted_origins" {
  description = "Exact origin the UI is served from, for the API's CSRF check. Empty disables the browser flows that need it."
  type        = string
  default     = ""
}

variable "domain" {
  description = "Hostname the edge serves. Cloudflare holds the zone; the certificate is regional because the ALB terminates TLS."
  type        = string
  default     = "godough.foonix.net"
}
