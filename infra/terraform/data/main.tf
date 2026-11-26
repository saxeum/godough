data "aws_availability_zones" "available" {
  state = "available"
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

# This is the security group the future EKS managed node group will use. The
# database accepts 5432 from this group and from nowhere else.
resource "aws_security_group" "eks_nodes" {
  name        = "${var.project}-eks-nodes"
  description = "EKS worker nodes; RDS accepts 5432 from this group"
  vpc_id      = aws_vpc.main.id

  egress {
    description = "VPC traffic; add NAT or endpoints before EKS needs the internet"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [var.vpc_cidr]
  }

  tags = {
    Name = "${var.project}-eks-nodes"
  }
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
