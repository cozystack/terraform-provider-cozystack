package client

// Additional Cozystack API groups beyond apps.cozystack.io and the cozystack.io
// platform group: backups, tenant core resources, and the dashboard.

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

// MarketplacePanelResource identifies the MarketplacePanel kind (cluster-scoped).
func MarketplacePanelResource() Resource {
	return Resource{Group: "dashboard.cozystack.io", Resource: "marketplacepanels", Kind: "MarketplacePanel", ClusterScoped: true}
}
