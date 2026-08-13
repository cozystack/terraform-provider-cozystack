data "cozystack_kubernetes" "cluster" {
  name      = "cluster"
  namespace = "tenant-root"
}

# The platform materialises its own defaults on read, so the effective Talos
# release is reported even for a cluster that never pinned one.
output "cluster_talos_version" {
  value = data.cozystack_kubernetes.cluster.talos.version
}

output "cluster_oidc_mode" {
  value = data.cozystack_kubernetes.cluster.oidc.mode
}
