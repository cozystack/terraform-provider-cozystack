resource "cozystack_nats" "bus" {
  name      = "bus"
  namespace = "tenant-root"

  replicas = 3
  tls      = { enabled = true }
  users    = { app = { password = "change-me" } }
}
