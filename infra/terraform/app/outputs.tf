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

output "certificate_arn" {
  description = "Region certificate for the ALB listener. Empty-looking until it is validated — see certificate_validation_record."
  value       = aws_acm_certificate.edge.arn
}

output "certificate_validation_records" {
  description = "Validation CNAMEs Terraform created in Cloudflare. Listed so they can be checked, not pasted."
  value       = { for name, record in cloudflare_dns_record.certificate_validation : name => record.name }
}

output "load_balancer_controller_role" {
  description = "IRSA role the controller uses; the ALB it creates is owned by the Ingress, not by this role."
  value       = aws_iam_role.lbc.arn
}

output "edge_hostname" {
  description = "Where godough.foonix.net points once alb_hostname is set."
  value       = var.alb_hostname == "" ? "(not set yet: apply the Ingress, then pass its hostname as alb_hostname)" : "CNAME godough.foonix.net -> ${var.alb_hostname} (grey)"
}
