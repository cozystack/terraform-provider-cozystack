resource "cozystack_opensearch" "search" {
  name      = "search"
  namespace = "tenant-root"

  replicas = 3
  version  = "v2"
  size     = "50Gi"

  users = {
    app = { password = "change-me", roles = ["admin"] }
  }
}
