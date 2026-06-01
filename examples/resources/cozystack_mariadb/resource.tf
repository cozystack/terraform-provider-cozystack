resource "cozystack_mariadb" "app" {
  name      = "app"
  namespace = "tenant-root"

  replicas = 2
  version  = "v11.8"
  size     = "20Gi"

  users = {
    app = { password = "change-me", max_user_connections = 100 }
  }
  databases = {
    appdb = { roles = { admin = ["app"] } }
  }
}
