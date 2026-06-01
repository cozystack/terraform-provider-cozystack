package client

// TenantResource describes the apps.cozystack.io Tenant resource.
func TenantResource() Resource {
	return Resource{Resource: "tenants", Kind: "Tenant"}
}
