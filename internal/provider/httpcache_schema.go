package provider

import (
	"maps"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func httpcacheSchema() rschema.Schema {
	attributes := identityResourceAttributes("HTTPCache instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrSize: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("10Gi"),
			MarkdownDescription: "Persistent volume size for cache data (quantity, e.g. `10Gi`).",
		},
		attrStorageClass: storageClassAttribute(""),
		attrExternal:     externalAttribute(),
		"endpoints": rschema.ListAttribute{
			Optional:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "Upstream endpoints as a list of `ip:port`.",
		},
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed HTTP cache, deployed inside a tenant namespace. " +
			"The haproxy and nginx tuning blocks use server defaults.",
		Attributes: attributes,
	}
}

func httpcacheDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("HTTPCache instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrSize:         dsschema.StringAttribute{Computed: true, MarkdownDescription: "Persistent volume size."},
		attrStorageClass: dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
		attrExternal:     dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
		"endpoints":      dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Upstream endpoints."},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack HTTP cache by name and namespace.",
		Attributes:          attributes,
	}
}
