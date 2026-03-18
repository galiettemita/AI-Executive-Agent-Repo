# ElastiCache Global Datastore — multi-region Redis (P3-09).

resource "aws_elasticache_global_replication_group" "brevio" {
  global_replication_group_id_suffix    = "brevio"
  primary_replication_group_id          = aws_elasticache_replication_group.primary.id
  global_replication_group_description  = "Brevio global Redis"
}

resource "aws_elasticache_replication_group" "primary" {
  provider                   = aws.us_east_1
  replication_group_id       = "brevio-primary"
  description                = "Brevio primary Redis"
  node_type                  = "cache.r7g.large"
  num_cache_clusters         = 2
  automatic_failover_enabled = true
  multi_az_enabled           = true
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true

  tags = { role = "primary", region = "us-east-1" }
}

resource "aws_elasticache_replication_group" "secondary" {
  provider                      = aws.eu_west_1
  replication_group_id          = "brevio-secondary"
  description                   = "Brevio EU Redis replica"
  global_replication_group_id   = aws_elasticache_global_replication_group.brevio.global_replication_group_id
  node_type                     = "cache.r7g.large"
  num_cache_clusters            = 1

  tags = { role = "secondary", region = "eu-west-1" }
}
