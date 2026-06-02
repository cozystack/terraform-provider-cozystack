resource "cozystack_kubernetes" "cluster" {
  name      = "cluster"
  namespace = "tenant-root"

  version       = "v1.35"
  storage_class = "replicated"

  node_groups = {
    md0 = {
      instance_type = "u1.medium"
      disk_size     = "20Gi"
      min_replicas  = 1
      max_replicas  = 10
      roles         = ["ingress-nginx"]
    }
  }
}
