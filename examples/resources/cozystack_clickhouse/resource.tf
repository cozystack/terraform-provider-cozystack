resource "cozystack_clickhouse" "analytics" {
  name      = "analytics"
  namespace = "tenant-root"

  replicas = 2
  shards   = 2
  size     = "50Gi"

  backup = { use_system_bucket = true }

  users = {
    reader = { password = "change-me", readonly = true }
  }
}
