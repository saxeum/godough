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

output "certificate_validation_record" {
  description = "The CNAME to add in Cloudflare so the certificate validates. Terraform cannot add it: no Cloudflare credentials here."
  value = {
    for option in aws_acm_certificate.edge.domain_validation_options : option.domain_name => {
      name  = option.resource_record_name
      type  = option.resource_record_type
      value = option.resource_record_value
    }
  }
}

output "load_balancer_controller_role" {
  description = "IRSA role the controller uses; the ALB it creates is owned by the Ingress, not by this role."
  value       = aws_iam_role.lbc.arn
}
