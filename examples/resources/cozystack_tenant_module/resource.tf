# A marker resource: its existence enables the module in the tenant.
resource "cozystack_tenant_module" "etcd" {
  name      = "etcd"
  namespace = "tenant-root"
}
