# BackupJob triggers a single backup run of a tenant application using the
# strategies defined by a BackupClass.
resource "cozystack_backup_job" "db_now" {
  name      = "db-manual-run"
  namespace = "tenant-root"
  spec = jsonencode({
    applicationRef = { apiGroup = "apps.cozystack.io", kind = "Postgres", name = "db" }
    # backupClassName is immutable server-side: changing it after creation is
    # rejected by the API. Replace the resource to point at a different class.
    backupClassName = "s3-daily"
  })
}
