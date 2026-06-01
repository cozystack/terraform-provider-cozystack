resource "cozystack_redis" "cache" {
  name      = "cache"
  namespace = "tenant-root"

  replicas     = 2
  size         = "2Gi"
  version      = "v8"
  auth_enabled = true
}

# Size with an explicit request instead of a preset.
resource "cozystack_redis" "sized" {
  name      = "sized-cache"
  namespace = "tenant-root"

  resources = {
    cpu    = "500m"
    memory = "512Mi"
  }
}
