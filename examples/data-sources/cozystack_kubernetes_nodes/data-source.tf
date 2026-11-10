data "cozystack_kubernetes_nodes" "gpu" {
  name      = "demo-gpu"
  namespace = "tenant-root"
}

output "worker_class_name" {
  value = data.cozystack_kubernetes_nodes.gpu.worker_class_name
}

output "ssh_keys" {
  value = data.cozystack_kubernetes_nodes.gpu.ssh_keys
}
