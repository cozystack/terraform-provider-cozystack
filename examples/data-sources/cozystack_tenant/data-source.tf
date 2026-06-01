data "cozystack_tenant" "root" {
  name      = "root"
  namespace = "tenant-root"
}

output "root_tenant_namespace" {
  value = data.cozystack_tenant.root.status_namespace
}
