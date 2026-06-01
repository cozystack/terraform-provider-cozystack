resource "cozystack_tenant" "team_a" {
  name      = "team-a"
  namespace = "tenant-root"

  monitoring = true
  ingress    = true

  resource_quotas = {
    cpu    = "8"
    memory = "16Gi"
  }
}

# Optionally block until the tenant is reconciled and Ready.
resource "cozystack_tenant" "team_b" {
  name      = "team-b"
  namespace = "tenant-root"

  wait_for_ready = true
  wait_timeout   = "15m"
}
