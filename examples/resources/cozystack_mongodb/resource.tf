resource "cozystack_mongodb" "app" {
  name      = "app"
  namespace = "tenant-root"

  replicas = 3
  version  = "v8"

  users     = { app = { password = "change-me" } }
  databases = { appdb = { roles = { admin = ["app"] } } }
}
