package provider

import (
	"maps"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
)

func natsSchema() rschema.Schema {
	attributes := identityResourceAttributes("NATS instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrReplicas: rschema.Int64Attribute{
			Optional: true, Computed: true,
			Default:             int64default.StaticInt64(2),
			MarkdownDescription: "Number of NATS replicas.",
		},
		attrResources:       resourcesResourceAttribute(),
		attrResourcesPreset: presetAttribute("t1.nano"),
		attrStorageClass:    storageClassAttribute(),
		attrExternal:        externalAttribute(),
		"users":             passwordUsersResourceAttribute("NATS users keyed by user name."),
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed NATS instance, deployed inside a tenant namespace. " +
			"The jetstream and config blocks use server defaults.",
		Attributes: attributes,
	}
}

func natsDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("NATS instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrReplicas:        dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Number of replicas."},
		attrResources:       resourcesDataSourceAttribute(),
		attrResourcesPreset: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
		attrStorageClass:    dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
		attrExternal:        dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
		"users":             passwordUsersDataSourceAttribute("NATS users keyed by user name."),
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack NATS instance by name and namespace.",
		Attributes:          attributes,
	}
}
