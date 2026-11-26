output "vpc_id" {
  description = "Staging VPC."
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "Public subnets, for a future internet-facing ALB."
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "Private subnets for RDS and the future EKS node group."
  value       = aws_subnet.private[*].id
}

output "eks_node_security_group_id" {
  description = "Security group the EKS managed node group must attach."
  value       = aws_security_group.eks_nodes.id
}

output "rds_security_group_id" {
  description = "Security group on the database."
  value       = aws_security_group.rds.id
}

output "rds_endpoint" {
  description = "RDS host:port."
  value       = aws_db_instance.postgres.endpoint
}

output "rds_port" {
  description = "RDS port."
  value       = aws_db_instance.postgres.port
}

output "rds_ca_cert_identifier" {
  description = "CA identifier used by verify-full clients."
  value       = var.db_ca_cert_identifier
}

output "master_user_secret_arn" {
  description = "Secrets Manager secret holding the RDS master credentials."
  value       = aws_db_instance.postgres.master_user_secret[0].secret_arn
}
