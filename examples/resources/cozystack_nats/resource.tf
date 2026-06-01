resource "cozystack_nats" "bus" {
  name      = "bus"
  namespace = "tenant-root"

  replicas = 3
  users    = { app = { password = "change-me" } }
}
