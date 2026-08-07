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

# gateway is three-state. Leave it out and the platform decides — on for a
# tenant whose apex derives from its parent, off for a custom apex. Set it to
# take that decision yourself: here a tenant with its own apex asks for a
# Gateway anyway.
resource "cozystack_tenant" "team_c" {
  name      = "team-c"
  namespace = "tenant-root"

  host    = "team-c.example.com"
  gateway = true
}
