output "namespace" {
  description = "Namespace both releases live in."
  value       = var.namespace
}

output "api_release" {
  description = "Helm release name for the api."
  value       = helm_release.api.name
}

output "ui_release" {
  description = "Helm release name for the ui."
  value       = helm_release.ui.name
}

output "api_digest" {
  description = "Digest deployed, so the running object can be compared with the reviewed one."
  value       = var.api_digest
}

output "ui_digest" {
  description = "Digest deployed for the ui."
  value       = var.ui_digest
}
