resource "cozystack_harbor" "registry" {
  name      = "registry"
  namespace = "tenant-root"

  host = "registry.example.com"
}
