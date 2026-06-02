resource "cozystack_package_source" "custom" {
  name = "custom.apps"
  spec = jsonencode({
    sourceRef = { kind = "OCIRepository", name = "custom-apps", namespace = "cozy-system" }
    variants  = [{ name = "default" }]
  })
}
