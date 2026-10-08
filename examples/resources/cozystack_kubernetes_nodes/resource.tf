resource "cozystack_kubernetes" "cluster" {
  name      = "demo"
  namespace = "tenant-root"

  version = "v1.35"
}

# A pool is named "<cluster>-<pool>".
resource "cozystack_kubernetes_nodes" "gpu" {
  name      = "${cozystack_kubernetes.cluster.name}-gpu"
  namespace = cozystack_kubernetes.cluster.namespace
  cluster   = cozystack_kubernetes.cluster.name

  version       = cozystack_kubernetes.cluster.version
  instance_type = "u1.xlarge"
  disk_size     = "100Gi"
  min_replicas  = 0
  max_replicas  = 4
  roles         = ["gpu"]

  gpus = [
    { name = "nvidia.com/AD102GL_L40S" },
  ]

  kubelet = {
    kube_reserved_memory = "1Gi"
  }
}
