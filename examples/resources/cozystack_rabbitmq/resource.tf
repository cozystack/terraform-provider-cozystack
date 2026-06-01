resource "cozystack_rabbitmq" "broker" {
  name      = "broker"
  namespace = "tenant-root"

  replicas = 3
  version  = "v4.2"

  users = {
    app = { password = "change-me" }
  }
  vhosts = {
    "/" = { roles = { admin = ["app"] } }
  }
}
