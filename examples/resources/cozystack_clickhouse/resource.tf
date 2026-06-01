resource "cozystack_clickhouse" "analytics" {
  name      = "analytics"
  namespace = "tenant-root"

  replicas = 2
  shards   = 2
  size     = "50Gi"

  users = {
    reader = { password = "change-me", readonly = true }
  }
}
