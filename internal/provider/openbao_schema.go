package provider

import (
	"maps"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

func openbaoSchema() rschema.Schema {
	attributes := identityResourceAttributes("OpenBAO instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrReplicas: rschema.Int64Attribute{
			Optional:            true,
			Computed:            true,
			Default:             int64default.StaticInt64(1),
			MarkdownDescription: "Number of OpenBAO replicas.",
		},
		attrSize: rschema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("10Gi"),
			MarkdownDescription: "Persistent volume size (quantity, e.g. `10Gi`).",
		},
		attrStorageClass: storageClassAttribute(""),
		attrExternal: rschema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Enable external access from outside the cluster.",
		},
		"ui": rschema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(true),
			MarkdownDescription: "Enable the OpenBAO web UI.",
		},
		attrResourcesPreset: rschema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("t1.small"),
			MarkdownDescription: "Sizing preset applied when `resources` is omitted (e.g. `t1.small`).",
		},
		attrResources: resourcesResourceAttribute(),
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed OpenBAO (Vault-compatible) instance, deployed inside a tenant namespace.",
		Attributes:          attributes,
	}
}

func openbaoDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("OpenBAO instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrReplicas:        dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Number of replicas."},
		attrSize:            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Persistent volume size."},
		attrStorageClass:    dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
		attrExternal:        dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
		"ui":                dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the web UI is enabled."},
		attrResourcesPreset: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
		attrResources:       resourcesDataSourceAttribute(),
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack OpenBAO instance by name and namespace.",
		Attributes:          attributes,
	}
}
