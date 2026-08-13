package client

// Additional Cozystack API groups beyond apps.cozystack.io and the cozystack.io
// platform group: backups and tenant core resources.

// BackupClassResource identifies the BackupClass kind (cluster-scoped).
func BackupClassResource() Resource {
	return Resource{Group: "backups.cozystack.io", Resource: "backupclasses", Kind: "BackupClass", ClusterScoped: true}
}

// BackupResource identifies the Backup kind (namespaced).
func BackupResource() Resource {
	return Resource{Group: "backups.cozystack.io", Resource: "backups", Kind: "Backup"}
}

// BackupJobResource identifies the BackupJob kind (namespaced).
func BackupJobResource() Resource {
	return Resource{Group: "backups.cozystack.io", Resource: "backupjobs", Kind: "BackupJob"}
}

// PlanResource identifies the backup Plan kind (namespaced).
func PlanResource() Resource {
	return Resource{Group: "backups.cozystack.io", Resource: "plans", Kind: "Plan"}
}

// RestoreJobResource identifies the RestoreJob kind (namespaced).
func RestoreJobResource() Resource {
	return Resource{Group: "backups.cozystack.io", Resource: "restorejobs", Kind: "RestoreJob"}
}

// TenantNamespaceResource identifies the TenantNamespace marker kind
// (cluster-scoped, no spec).
func TenantNamespaceResource() Resource {
	return Resource{
		Group: "core.cozystack.io", Resource: "tenantnamespaces", Kind: "TenantNamespace",
		ClusterScoped: true, NoSpec: true,
	}
}

// TenantModuleResource identifies the TenantModule marker kind (namespaced, no spec).
func TenantModuleResource() Resource {
	return Resource{
		Group: "core.cozystack.io", Resource: "tenantmodules", Kind: "TenantModule", NoSpec: true,
	}
}

// TenantSecretResource identifies the TenantSecret kind (namespaced, Secret-shaped).
func TenantSecretResource() Resource {
	return Resource{Group: "core.cozystack.io", Resource: "tenantsecrets", Kind: "TenantSecret"}
}
