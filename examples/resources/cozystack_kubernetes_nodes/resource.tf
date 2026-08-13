resource "cozystack_kubernetes" "cluster" {
  name      = "demo"
  namespace = "tenant-root"

  version = "v1.35"

  node_groups = {
    md0 = {
      instance_type = "u1.medium"
      min_replicas  = 1
      max_replicas  = 3
    }
  }
}

# A standalone pool is named "<cluster>-<pool>". The pool part must not collide
# with a node group the parent cluster still manages, such as md0 above.
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
