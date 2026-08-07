resource "cozystack_postgres" "app" {
  name      = "app"
  namespace = "tenant-root"

  replicas = 2
  version  = "v18"
  size     = "20Gi"
  tls      = { enabled = true }

  users = {
    app = { password = "change-me" }
  }
  databases = {
    appdb = {
      extensions = ["postgis"]
      roles      = { admin = ["app"] }
    }
  }
}
