package provider

import (
	"maps"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func markerSchema(schemaDesc, nameDesc string) rschema.Schema {
	attributes := clusterIdentityResourceAttributes(nameDesc)
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{MarkdownDescription: schemaDesc, Attributes: attributes}
}

func markerDataSourceSchema(schemaDesc, nameDesc string) dsschema.Schema {
	attributes := clusterIdentityDataSourceAttributes(nameDesc)
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{MarkdownDescription: schemaDesc, Attributes: attributes}
}

func markerNsSchema(schemaDesc, nameDesc string) rschema.Schema {
	attributes := identityResourceAttributes(nameDesc)
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{MarkdownDescription: schemaDesc, Attributes: attributes}
}

func markerNsDataSourceSchema(schemaDesc, nameDesc string) dsschema.Schema {
	attributes := identityDataSourceAttributes(nameDesc)
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{MarkdownDescription: schemaDesc, Attributes: attributes}
}

func tenantNamespaceSchema() rschema.Schema {
	return markerSchema("A Cozystack TenantNamespace (cluster-scoped marker; its existence by name "+
		"provisions the tenant namespace).", "TenantNamespace name. Immutable.")
}

func tenantNamespaceDataSourceSchema() dsschema.Schema {
	return markerDataSourceSchema("Read a Cozystack TenantNamespace by name.", "TenantNamespace name.")
}

func tenantModuleSchema() rschema.Schema {
	return markerNsSchema("A Cozystack TenantModule (marker; its existence by name enables the module "+
		"in the tenant).", "TenantModule name (the module, e.g. `etcd`). Immutable.")
}

func tenantModuleDataSourceSchema() dsschema.Schema {
	return markerNsDataSourceSchema("Read a Cozystack TenantModule by name and namespace.", "TenantModule name.")
}
