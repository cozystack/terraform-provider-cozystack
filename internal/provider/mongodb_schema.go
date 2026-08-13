package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func mongodbSchema() rschema.Schema {
	attributes := identityResourceAttributes("MongoDB instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrReplicas: rschema.Int64Attribute{
			Optional: true, Computed: true,
			Default:             int64default.StaticInt64(3),
			MarkdownDescription: "Number of MongoDB replicas.",
		},
		attrResources:       resourcesResourceAttribute(),
		attrResourcesPreset: presetAttribute("t1.small"),
		attrSize: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("10Gi"),
			MarkdownDescription: "Persistent volume size (quantity, e.g. `10Gi`).",
		},
		attrStorageClass: storageClassAttribute(""),
		attrExternal:     externalAttribute(),
		attrVersion: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("v8"),
			Validators:          []validator.String{stringvalidator.OneOf("v8", "v7", "v6")},
			MarkdownDescription: "MongoDB major version (`v8`, `v7`, `v6`).",
		},
		"sharding": rschema.BoolAttribute{
			Optional: true, Computed: true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Enable sharding (detailed shardingConfig uses server defaults).",
		},
		"users":     passwordUsersResourceAttribute("MongoDB users keyed by user name."),
		"databases": rolesMapResourceAttribute("Databases keyed by name, each with admin/readonly user roles."),
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed MongoDB instance, deployed inside a tenant namespace. " +
			"The deprecated backup block, the bootstrap restore block, and detailed shardingConfig are not managed.",
		Attributes: attributes,
	}
}

func mongodbDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("MongoDB instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrReplicas:        dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Number of replicas."},
		attrResources:       resourcesDataSourceAttribute(),
		attrResourcesPreset: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
		attrSize:            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Persistent volume size."},
		attrStorageClass:    dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
		attrExternal:        dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
		attrVersion:         dsschema.StringAttribute{Computed: true, MarkdownDescription: "MongoDB major version."},
		"sharding":          dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether sharding is enabled."},
		"users":             passwordUsersDataSourceAttribute("MongoDB users keyed by user name."),
		"databases":         rolesMapDataSourceAttribute("Databases keyed by name."),
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack MongoDB instance by name and namespace.",
		Attributes:          attributes,
	}
}
