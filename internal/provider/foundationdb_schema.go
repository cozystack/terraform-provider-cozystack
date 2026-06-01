package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fdbStorageResourceAttribute() rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		Optional: true, Computed: true,
		Default:             objectdefault.StaticValue(fdbStorageDefault()),
		MarkdownDescription: "Persistent storage configuration.",
		Attributes: map[string]rschema.Attribute{
			"size": rschema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString("16Gi"),
				MarkdownDescription: "Size of persistent volumes for each instance.",
			},
			"storage_class": rschema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "StorageClass used to store the data.",
			},
		},
	}
}

func foundationdbSchema() rschema.Schema {
	attributes := identityResourceAttributes("FoundationDB instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		"storage":           fdbStorageResourceAttribute(),
		attrResources:       resourcesResourceAttribute(),
		attrResourcesPreset: presetAttribute("c1.small"),
		"custom_parameters": rschema.ListAttribute{
			Optional:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "Custom parameters passed to FoundationDB.",
		},
		"image_type": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("unified"),
			Validators:          []validator.String{stringvalidator.OneOf("unified", "split")},
			MarkdownDescription: "Container image deployment type (`unified` or `split`).",
		},
		"automatic_replacements": rschema.BoolAttribute{
			Optional: true, Computed: true,
			Default:             booldefault.StaticBool(true),
			MarkdownDescription: "Enable automatic pod replacements.",
		},
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed FoundationDB cluster, deployed inside a tenant namespace. " +
			"The cluster topology, deprecated backup, monitoring, and securityContext blocks use server defaults.",
		Attributes: attributes,
	}
}

func foundationdbDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("FoundationDB instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		"storage": dsschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Persistent storage configuration.",
			Attributes: map[string]dsschema.Attribute{
				"size":          dsschema.StringAttribute{Computed: true, MarkdownDescription: "Size of persistent volumes."},
				"storage_class": dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
			},
		},
		attrResources:            resourcesDataSourceAttribute(),
		attrResourcesPreset:      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
		"custom_parameters":      dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Custom parameters."},
		"image_type":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "Container image deployment type."},
		"automatic_replacements": dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether automatic pod replacements are enabled."},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack FoundationDB cluster by name and namespace.",
		Attributes:          attributes,
	}
}
