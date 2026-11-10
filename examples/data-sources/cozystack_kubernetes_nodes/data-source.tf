data "cozystack_kubernetes_nodes" "gpu" {
  name      = "demo-gpu"
  namespace = "tenant-root"
}

output "kubernetes_nodes_gpus" {
  value = data.cozystack_kubernetes_nodes.gpu.gpus
}
