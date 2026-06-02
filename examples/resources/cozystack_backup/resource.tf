# Backup is a namespaced record of a backup that was taken for a tenant
# application. These objects are normally produced by the platform; manage one
# declaratively only when registering an externally-taken backup.
resource "cozystack_backup" "db_snapshot" {
  name      = "db-20260602"
  namespace = "tenant-root"
  spec = jsonencode({
    applicationRef = { apiGroup = "apps.cozystack.io", kind = "Postgres", name = "db" }
    strategyRef    = { apiGroup = "strategy.backups.cozystack.io", kind = "PostgresBackupStrategy", name = "s3" }
    takenAt        = "2026-06-02T03:00:00Z"
  })
}
