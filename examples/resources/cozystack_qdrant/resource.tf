resource "cozystack_qdrant" "vectors" {
  name      = "vectors"
  namespace = "tenant-root"

  replicas = 2
  size     = "20Gi"
  tls      = { enabled = true }
}
