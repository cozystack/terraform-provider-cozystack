data "cozystack_kubernetes_nodes" "gpu" {
  name      = "demo-gpu"
  namespace = "tenant-root"
}
