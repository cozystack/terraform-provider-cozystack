package provider

import (
	"maps"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func harborSchema() rschema.Schema {
	attributes := identityResourceAttributes("Harbor instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrHost: rschema.StringAttribute{
			Optional: true, Computed: true,
			MarkdownDescription: "Hostname for external access to Harbor. " +
				"Defaults to a `harbor` subdomain of the tenant host.",
		},
		attrStorageClass: storageClassAttribute(),
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed Harbor registry, deployed inside a tenant namespace. " +
			"The core, registry, jobservice, trivy, database, and redis component blocks use server defaults.",
		Attributes: attributes,
	}
}

func harborDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("Harbor instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrHost:         dsschema.StringAttribute{Computed: true, MarkdownDescription: "Hostname for external access to Harbor."},
		attrStorageClass: dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack Harbor registry by name and namespace.",
		Attributes:          attributes,
	}
}
