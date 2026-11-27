data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-kernel-*-arm64"]
  }

  filter {
    name   = "architecture"
    values = ["arm64"]
  }

  filter {
    name   = "state"
    values = ["available"]
  }
}

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${var.project}-staging"
  }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.project}-staging"
  }
}

resource "aws_subnet" "public" {
  count = 2

  vpc_id                  = aws_vpc.main.id
  cidr_block              = cidrsubnet(var.vpc_cidr, 8, count.index)
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = false

  tags = {
    Name = "${var.project}-public-${count.index + 1}"
    Tier = "public"

    # How the AWS Load Balancer Controller finds subnets to put an internet-facing
    # ALB in. Without the role tag it reports "unable to discover subnets" and no
    # ALB is created; without the cluster tag it ignores the subnet entirely.
    "kubernetes.io/role/elb"                       = "1"
    "kubernetes.io/cluster/${var.project}-staging" = "shared"
  }
}

resource "aws_subnet" "private" {
  count = 2

  vpc_id                  = aws_vpc.main.id
  cidr_block              = cidrsubnet(var.vpc_cidr, 8, count.index + 10)
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = false

  tags = {
    Name = "${var.project}-private-${count.index + 1}"
    Tier = "private"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "${var.project}-public"
  }
}

resource "aws_route_table_association" "public" {
  count = 2

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.project}-private"
  }
}

resource "aws_route_table_association" "private" {
  count = 2

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

# One NAT gateway in the first public subnet, for the EKS nodes: image pulls from
# ECR, the STS and EC2 APIs the node and the volume controller call, and the
# cluster endpoint. The alternative is five interface endpoints at about the same
# monthly cost, and this is one address to reason about.
#
# Single-AZ is the trade: if public-1 or its NAT fails, running pods keep running
# and RDS is untouched, but nothing new can pull an image or reach the API. A NAT
# per AZ would be about another $33/month to remove that, which staging does not
# need yet.
resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${var.project}-nat"
  }
}

resource "aws_nat_gateway" "main" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id

  tags = {
    Name = "${var.project}-nat"
  }

  depends_on = [aws_internet_gateway.main]
}

resource "aws_route" "private_nat" {
  route_table_id         = aws_route_table.private.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.main.id
}

# This is the security group the future EKS managed node group will use. The
# database accepts 5432 from this group and from nowhere else.
resource "aws_security_group" "eks_nodes" {
  name        = "${var.project}-eks-nodes"
  description = "EKS worker nodes; RDS accepts 5432 from this group"
  vpc_id      = aws_vpc.main.id

  egress {
    description = "VPC traffic: RDS, CoreDNS and the cluster endpoint ENIs"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [var.vpc_cidr]
  }

  egress {
    description = "ECR, STS, EC2 and the EKS API, through the NAT gateway"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    #trivy:ignore:AVD-AWS-0104
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project}-eks-nodes"
  }
}

resource "aws_security_group" "bastion" {
  name        = "${var.project}-ssm-bastion"
  description = "SSM-only bastion; no inbound rules"
  vpc_id      = aws_vpc.main.id

  egress {
    description = "SSM control plane"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    # Egress to the SSM control plane is why this is the only 0.0.0.0/0 rule.
    # There is no inbound rule and no SSH. The alternative is three interface
    # endpoints at roughly $21/month for a break-glass bastion.
    #trivy:ignore:AVD-AWS-0104
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "Postgres in the VPC for port forwarding"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  tags = {
    Name = "${var.project}-ssm-bastion"
  }
}

data "aws_iam_policy_document" "bastion_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "bastion" {
  name               = "${var.project}-ssm-bastion"
  assume_role_policy = data.aws_iam_policy_document.bastion_assume.json
}

resource "aws_iam_role_policy_attachment" "bastion_ssm" {
  role       = aws_iam_role.bastion.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "bastion" {
  name = "${var.project}-ssm-bastion"
  role = aws_iam_role.bastion.name
}

resource "aws_instance" "bastion" {
  ami                         = data.aws_ami.al2023.id
  instance_type               = "t4g.nano"
  subnet_id                   = aws_subnet.public[0].id
  vpc_security_group_ids      = [aws_security_group.bastion.id]
  iam_instance_profile        = aws_iam_instance_profile.bastion.name
  associate_public_ip_address = true

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
  }

  root_block_device {
    volume_size = 8
    volume_type = "gp3"
    encrypted   = true
  }

  user_data = <<-EOF
    #!/bin/bash
    systemctl enable --now amazon-ssm-agent
  EOF

  tags = {
    Name = "${var.project}-ssm-bastion"
  }

  depends_on = [aws_route_table_association.public]
}

resource "aws_security_group" "rds" {
  name        = "${var.project}-rds"
  description = "Postgres; only the EKS node security group may connect"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Postgres from EKS nodes"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_nodes.id]
  }

  ingress {
    description     = "Postgres from the SSM bastion"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.bastion.id]
  }

  tags = {
    Name = "${var.project}-rds"
  }
}

resource "aws_db_subnet_group" "main" {
  name       = "${var.project}-staging"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "${var.project}-staging"
  }
}

resource "aws_db_parameter_group" "postgres" {
  name   = "${var.project}-postgres17"
  family = "postgres17"

  parameter {
    name         = "rds.force_ssl"
    value        = "1"
    apply_method = "pending-reboot"
  }

  tags = {
    Name = "${var.project}-postgres17"
  }
}

# RDS owns the master password in Secrets Manager. Terraform never receives it,
# so it is not in state or a plan file.
resource "aws_db_instance" "postgres" {
  identifier = "${var.project}-staging"

  engine         = "postgres"
  engine_version = var.db_engine_version
  instance_class = var.db_instance_class

  db_name  = "godough"
  username = "godough"

  manage_master_user_password = true

  allocated_storage     = 20
  max_allocated_storage = 100
  storage_type          = "gp3"
  storage_encrypted     = true

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  parameter_group_name   = aws_db_parameter_group.postgres.name
  publicly_accessible    = false
  multi_az               = false

  ca_cert_identifier = var.db_ca_cert_identifier

  backup_retention_period    = 7
  apply_immediately          = true
  auto_minor_version_upgrade = true

  deletion_protection   = false
  skip_final_snapshot   = true
  copy_tags_to_snapshot = true

  tags = {
    Name        = "${var.project}-staging"
    Environment = "staging"
  }
}
