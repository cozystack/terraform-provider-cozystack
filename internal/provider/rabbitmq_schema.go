package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func rabbitmqSchema() rschema.Schema {
	attributes := identityResourceAttributes("RabbitMQ instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrReplicas: rschema.Int64Attribute{
			Optional: true, Computed: true,
			Default:             int64default.StaticInt64(3),
			MarkdownDescription: "Number of RabbitMQ replicas.",
		},
		attrResources:       resourcesResourceAttribute(),
		attrResourcesPreset: presetAttribute("s1.nano"),
		attrSize: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("10Gi"),
			MarkdownDescription: "Persistent volume size (quantity, e.g. `10Gi`).",
		},
		attrStorageClass: storageClassAttribute(""),
		attrExternal:     externalAttribute(),
		attrVersion: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("v4.2"),
			Validators:          []validator.String{stringvalidator.OneOf("v4.2", "v4.1", "v4.0", "v3.13")},
			MarkdownDescription: "RabbitMQ major version (`v4.2`, `v4.1`, `v4.0`, `v3.13`).",
		},
		"users":  passwordUsersResourceAttribute("RabbitMQ users keyed by user name."),
		"vhosts": rolesMapResourceAttribute("Virtual hosts keyed by name, each with admin/readonly user roles."),
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed RabbitMQ instance, deployed inside a tenant namespace.",
		Attributes:          attributes,
	}
}

func rabbitmqDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("RabbitMQ instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrReplicas:        dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Number of replicas."},
		attrResources:       resourcesDataSourceAttribute(),
		attrResourcesPreset: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
		attrSize:            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Persistent volume size."},
		attrStorageClass:    dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
		attrExternal:        dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
		attrVersion:         dsschema.StringAttribute{Computed: true, MarkdownDescription: "RabbitMQ major version."},
		"users":             passwordUsersDataSourceAttribute("RabbitMQ users keyed by user name."),
		"vhosts":            rolesMapDataSourceAttribute("Virtual hosts keyed by name."),
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack RabbitMQ instance by name and namespace.",
		Attributes:          attributes,
	}
}
