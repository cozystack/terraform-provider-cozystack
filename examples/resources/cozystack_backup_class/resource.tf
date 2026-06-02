# BackupClass is a cluster-scoped resource (backups.cozystack.io group). It
# names a set of backup strategies, each tying an application kind to a
# driver-specific BackupStrategy.
resource "cozystack_backup_class" "s3_daily" {
  name = "s3-daily"
  spec = jsonencode({
    strategies = [
      {
        application = { apiGroup = "apps.cozystack.io", kind = "Postgres" }
        strategyRef = { apiGroup = "strategy.backups.cozystack.io", kind = "PostgresBackupStrategy", name = "s3" }
        parameters  = { schedule = "0 3 * * *", retention = "7d" }
      },
    ]
  })
}
