resource "cozystack_restore_job" "restore" {
  name        = "restore-app"
  namespace   = "tenant-root"
  backup_name = "daily-20260602030000"

  target_application_ref = {
    api_group = "apps.cozystack.io"
    kind      = "Postgres"
    name      = "app"
  }
}
