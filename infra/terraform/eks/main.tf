data "aws_caller_identity" "current" {}

# The VPC, subnets, NAT gateway and the node security group come from the data
# stack. They are read from its state rather than recreated here, so there is one
# VPC and one answer to "which subnets are private".
data "terraform_remote_state" "data" {
  backend = "s3"

  config = {
    bucket = "${var.project}-terraform-state-${data.aws_caller_identity.current.account_id}"
    key    = "data/terraform.tfstate"
    region = var.aws_region
  }
}

locals {
  cluster_name        = "${var.project}-staging"
  private_subnet_ids  = data.terraform_remote_state.data.outputs.private_subnet_ids
  node_security_group = data.terraform_remote_state.data.outputs.eks_node_security_group_id

  # The issuer URL is what IRSA trust policies are keyed on, so it is used twice:
  # once as the provider URL, once as the condition prefix.
  oidc_issuer = aws_eks_cluster.this.identity[0].oidc[0].issuer
  oidc_host   = replace(local.oidc_issuer, "https://", "")
}

data "aws_iam_policy_document" "cluster_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "cluster" {
  name               = "${local.cluster_name}-cluster"
  assume_role_policy = data.aws_iam_policy_document.cluster_assume.json
}

resource "aws_iam_role_policy_attachment" "cluster" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

data "aws_iam_policy_document" "nodes_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "nodes" {
  name               = "${local.cluster_name}-nodes"
  assume_role_policy = data.aws_iam_policy_document.nodes_assume.json
}

# WorkerNodePolicy: register with the cluster. CNI_Policy: assign pod addresses.
# ContainerRegistryReadOnly: pull from ECR without a static credential.
resource "aws_iam_role_policy_attachment" "nodes" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
  ])

  role       = aws_iam_role.nodes.name
  policy_arn = each.value
}

# Kubernetes Secrets are encrypted in etcd with this key rather than only with
# the volume's default encryption. Not in #117's list, but the Trivy config gate
# fails a cluster without it at HIGH, and this cluster is where the database
# credentials land in #114/#115 — the wrong place to suppress a finding.
#
# The key cannot be removed later: EKS refuses to disable envelope encryption on
# an existing cluster.
data "aws_iam_policy_document" "secrets_key" {
  statement {
    sid = "AllowTheClusterRole"

    actions = [
      "kms:CreateGrant",
      "kms:Decrypt",
      "kms:DescribeKey",
      "kms:Encrypt",
    ]

    principals {
      type        = "AWS"
      identifiers = [aws_iam_role.cluster.arn]
    }

    resources = ["*"]
  }

  # A custom policy replaces the default, so the account root statement has to be
  # written back: without it the key is not administrable by anyone.
  statement {
    sid     = "AllowAccountAdministration"
    actions = ["kms:*"]

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"]
    }

    resources = ["*"]
  }
}

resource "aws_kms_key" "secrets" {
  description         = "${local.cluster_name} Kubernetes Secret encryption"
  enable_key_rotation = true
  policy              = data.aws_iam_policy_document.secrets_key.json

  tags = {
    Name = "${local.cluster_name}-secrets"
  }
}

resource "aws_kms_alias" "secrets" {
  name          = "alias/${local.cluster_name}-secrets"
  target_key_id = aws_kms_key.secrets.key_id
}

resource "aws_eks_cluster" "this" {
  name     = local.cluster_name
  role_arn = aws_iam_role.cluster.arn
  version  = var.cluster_version

  vpc_config {
    # Private only, like RDS. kubectl goes through the SSM bastion over a port
    # forward, which is the access path that already exists for psql; see
    # README.md. A public endpoint with an allowlist was the alternative and it
    # solves nothing: CI has no stable egress address to allowlist, so #115 needs
    # either this tunnel or a runner inside the VPC either way.
    subnet_ids              = local.private_subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = false
  }

  # Without bootstrap_cluster_creator_admin_permissions the cluster is created
  # with no access entries, and the first kubectl call fails with Unauthorized —
  # which reads like bad credentials rather than an empty access list. It grants
  # the applying identity admin.
  access_config {
    authentication_mode                         = "API_AND_CONFIG_MAP"
    bootstrap_cluster_creator_admin_permissions = true
  }

  encryption_config {
    provider {
      key_arn = aws_kms_key.secrets.arn
    }

    resources = ["secrets"]
  }

  depends_on = [aws_iam_role_policy_attachment.cluster]
}

# IRSA. The cluster's issuer is trusted through this provider, and each workload
# role below trusts one service account in one namespace.
data "tls_certificate" "cluster" {
  url = local.oidc_issuer
}

resource "aws_iam_openid_connect_provider" "cluster" {
  url             = local.oidc_issuer
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.cluster.certificates[0].sha1_fingerprint]
}

data "aws_iam_policy_document" "ebs_csi_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.cluster.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:sub"
      values   = ["system:serviceaccount:kube-system:ebs-csi-controller-sa"]
    }
  }
}

# The volume controller calls EC2, so it gets its own role rather than a policy on
# the node role: on the node role every pod in the cluster could delete volumes.
resource "aws_iam_role" "ebs_csi" {
  name               = "${local.cluster_name}-ebs-csi"
  assume_role_policy = data.aws_iam_policy_document.ebs_csi_assume.json
}

resource "aws_iam_role_policy_attachment" "ebs_csi" {
  role       = aws_iam_role.ebs_csi.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
}

# A launch template is the only way to attach an extra security group to a
# managed node group. The group is the data stack's, which RDS already trusts on
# 5432, so the database needs no new rule.
resource "aws_launch_template" "nodes" {
  name_prefix = "${local.cluster_name}-node-"

  vpc_security_group_ids = [local.node_security_group]

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
    # Pods that use the instance profile (the VPC CNI does) need a second hop to
    # reach IMDS, which is otherwise only available to the host.
    http_put_response_hop_limit = 2
  }

  tag_specifications {
    resource_type = "instance"

    tags = {
      Name = "${local.cluster_name}-node"
    }
  }
}

resource "aws_eks_node_group" "staging" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "staging"
  node_role_arn   = aws_iam_role.nodes.arn
  subnet_ids      = local.private_subnet_ids

  # No image_id: the group takes the EKS-optimised AL2023 ARM64 image that
  # matches its cluster version, so an AMI upgrade is a node group update rather
  # than a pinned id that goes stale.
  ami_type       = "AL2023_ARM_64_STANDARD"
  instance_types = var.node_instance_types
  capacity_type  = "ON_DEMAND"

  scaling_config {
    desired_size = var.node_desired_size
    min_size     = var.node_min_size
    max_size     = var.node_max_size
  }

  # One node at a time: with max 2 there is room to replace the running node
  # before the old one goes.
  update_config {
    max_unavailable = 1
  }

  launch_template {
    id      = aws_launch_template.nodes.id
    version = aws_launch_template.nodes.latest_version
  }

  depends_on = [aws_iam_role_policy_attachment.nodes]
}

data "aws_eks_addon_version" "vpc_cni" {
  addon_name         = "vpc-cni"
  kubernetes_version = aws_eks_cluster.this.version
  most_recent        = true
}

data "aws_eks_addon_version" "kube_proxy" {
  addon_name         = "kube-proxy"
  kubernetes_version = aws_eks_cluster.this.version
  most_recent        = true
}

data "aws_eks_addon_version" "coredns" {
  addon_name         = "coredns"
  kubernetes_version = aws_eks_cluster.this.version
  most_recent        = true
}

data "aws_eks_addon_version" "ebs_csi" {
  addon_name         = "aws-ebs-csi-driver"
  kubernetes_version = aws_eks_cluster.this.version
  most_recent        = true
}

# The three that need no permissions of their own: they run with the node role.
# Left to the cluster, not to a chart, so their versions track the control plane.
resource "aws_eks_addon" "base" {
  for_each = {
    vpc-cni    = data.aws_eks_addon_version.vpc_cni.version
    kube-proxy = data.aws_eks_addon_version.kube_proxy.version
    coredns    = data.aws_eks_addon_version.coredns.version
  }

  cluster_name  = aws_eks_cluster.this.name
  addon_name    = each.key
  addon_version = each.value

  # An addon reports healthy only once a node can schedule it, so this waits for
  # the node group instead of racing it into DEGRADED.
  depends_on = [aws_eks_node_group.staging]
}

resource "aws_eks_addon" "ebs_csi" {
  cluster_name             = aws_eks_cluster.this.name
  addon_name               = "aws-ebs-csi-driver"
  addon_version            = data.aws_eks_addon_version.ebs_csi.version
  service_account_role_arn = aws_iam_role.ebs_csi.arn

  depends_on = [
    aws_eks_node_group.staging,
    aws_iam_role_policy_attachment.ebs_csi,
  ]
}
