# Package is a cluster-scoped platform resource (cozystack.io group).
resource "cozystack_package" "monitoring" {
  name    = "cozystack.monitoring"
  variant = "default"

  # Optional per-component overrides as JSON.
  components = jsonencode({
    grafana = { enabled = true }
  })
}
