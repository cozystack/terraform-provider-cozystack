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

# Leave gateway out and the tenant gets no Gateway of its own: it inherits the
# nearest ancestor's, or falls back to Ingress when no ancestor owns one. A
# tenant with a custom apex has to ask explicitly, because the ancestor's
# certificate does not cover that apex.
resource "cozystack_tenant" "team_c" {
  name      = "team-c"
  namespace = "tenant-root"

  host    = "team-c.example.com"
  gateway = true
}
