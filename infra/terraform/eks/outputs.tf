output "cluster_name" {
  description = "EKS cluster name, for aws eks update-kubeconfig."
  value       = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  description = "Kubernetes API endpoint."
  value       = aws_eks_cluster.this.endpoint
}

output "cluster_certificate_authority" {
  description = "Cluster CA, base64. kubeconfig carries it; -status and tls-check do not need it."
  value       = aws_eks_cluster.this.certificate_authority[0].data
}

output "cluster_version" {
  description = "Kubernetes version actually running, which may be newer than the variable after an EKS upgrade."
  value       = aws_eks_cluster.this.version
}

output "oidc_provider_arn" {
  description = "IRSA provider. Workload roles in #114 and #115 trust it."
  value       = aws_iam_openid_connect_provider.cluster.arn
}

output "oidc_issuer" {
  description = "Cluster OIDC issuer URL."
  value       = local.oidc_issuer
}

output "node_security_group_id" {
  description = "Security group the nodes run in. RDS accepts 5432 from it."
  value       = local.node_security_group
}

output "node_role_arn" {
  description = "Instance role for the nodes."
  value       = aws_iam_role.nodes.arn
}

output "ebs_csi_role_arn" {
  description = "IRSA role for the EBS CSI controller."
  value       = aws_iam_role.ebs_csi.arn
}
