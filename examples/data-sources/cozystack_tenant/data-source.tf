data "cozystack_tenant" "root" {
  name      = "root"
  namespace = "tenant-root"
}

output "root_tenant_namespace" {
  value = data.cozystack_tenant.root.status_namespace
}

# Null while the tenant leaves the decision to the platform.
output "root_tenant_gateway" {
  value = data.cozystack_tenant.root.gateway
}
