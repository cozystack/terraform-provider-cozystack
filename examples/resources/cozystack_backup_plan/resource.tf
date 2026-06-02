resource "cozystack_backup_plan" "daily" {
  name              = "daily"
  namespace         = "tenant-root"
  backup_class_name = "s3-daily"

  application_ref = {
    api_group = "apps.cozystack.io"
    kind      = "Postgres"
    name      = "app"
  }

  schedule = {
    cron = "0 3 * * *"
    type = "Cron"
  }
}
