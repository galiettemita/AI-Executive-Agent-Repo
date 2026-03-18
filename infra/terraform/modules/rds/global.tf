# Aurora Global Database — multi-region active-active (P3-09).

variable "db_username" {
  type      = string
  default   = "brevio_admin"
  sensitive = true
}

variable "db_password" {
  type      = string
  default   = ""
  sensitive = true
}

resource "aws_rds_global_cluster" "brevio" {
  global_cluster_identifier = "brevio-global"
  engine                    = "aurora-postgresql"
  engine_version            = "16.1"
  database_name             = "brevio"
  storage_encrypted         = true
}

resource "aws_rds_cluster" "primary" {
  provider                  = aws.us_east_1
  cluster_identifier        = "brevio-primary"
  engine                    = "aurora-postgresql"
  engine_version            = "16.1"
  global_cluster_identifier = aws_rds_global_cluster.brevio.id
  database_name             = "brevio"
  master_username           = var.db_username
  master_password           = var.db_password
  storage_encrypted         = true
  backup_retention_period   = 7
  deletion_protection       = true
  skip_final_snapshot       = false

  tags = { role = "primary", region = "us-east-1" }
}

resource "aws_rds_cluster" "secondary" {
  provider                  = aws.eu_west_1
  cluster_identifier        = "brevio-secondary"
  engine                    = "aurora-postgresql"
  engine_version            = "16.1"
  global_cluster_identifier = aws_rds_global_cluster.brevio.id
  storage_encrypted         = true

  lifecycle {
    ignore_changes = [replication_source_identifier]
  }

  tags = { role = "secondary", region = "eu-west-1" }
}
