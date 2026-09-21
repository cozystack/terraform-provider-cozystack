resource "cozystack_postgres" "app" {
  name      = "app"
  namespace = "tenant-root"

  replicas = 2
  version  = "v18"
  size     = "20Gi"
  tls      = { enabled = true }

  backup = { use_system_bucket = true }

  users = {
    app = { password = "change-me" }
  }
  databases = {
    appdb = {
      extensions = ["postgis"]
      roles      = { admin = ["app"] }
    }
  }

  # The connection endpoints are published asynchronously, so without waiting
  # they are only readable from the apply after the one that created the cluster.
  wait_for_ready = true
}
