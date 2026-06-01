package provider

import (
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Shared schema fragments reused by every resource/data source kind.

// presetAttribute returns the optional/computed resources_preset attribute.
func presetAttribute(def string) rschema.StringAttribute {
	return rschema.StringAttribute{
		Optional:            true,
		Computed:            true,
		Default:             stringdefault.StaticString(def),
		MarkdownDescription: "Sizing preset applied when `resources` is omitted.",
	}
}

// storageClassAttribute returns the optional/computed storage_class attribute.
func storageClassAttribute() rschema.StringAttribute {
	return rschema.StringAttribute{
		Optional:            true,
		Computed:            true,
		Default:             stringdefault.StaticString(""),
		MarkdownDescription: "StorageClass used to store the data.",
	}
}

// externalAttribute returns the optional/computed external attribute.
func externalAttribute() rschema.BoolAttribute {
	return rschema.BoolAttribute{
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(false),
		MarkdownDescription: "Enable external access from outside the cluster.",
	}
}

// identityResourceAttributes returns the id/name/namespace attributes common to
// every resource schema.
func identityResourceAttributes(nameDesc string) map[string]rschema.Attribute {
	return map[string]rschema.Attribute{
		attrID: rschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Synthetic identifier in the form `namespace/name`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		attrName: rschema.StringAttribute{
			Required:            true,
			MarkdownDescription: nameDesc,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		attrNamespace: rschema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Tenant namespace the application is created in. Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
	}
}

// statusResourceAttributes returns the computed ready/chart_version attributes.
func statusResourceAttributes() map[string]rschema.Attribute {
	return map[string]rschema.Attribute{
		attrReady: rschema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the application's `Ready` condition is true.",
		},
		attrChartVersion: rschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Deployed chart version (`status.version`).",
		},
	}
}

// resourcesResourceAttribute returns the optional resources{cpu,memory} block.
func resourcesResourceAttribute() rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Explicit CPU and memory per replica; overrides `resources_preset` for any field set.",
		Attributes: map[string]rschema.Attribute{
			attrCPU: rschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "CPU available to each replica (quantity, e.g. `500m`).",
			},
			attrMemory: rschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Memory available to each replica (quantity, e.g. `512Mi`).",
			},
		},
	}
}

// identityDataSourceAttributes returns the id/name/namespace attributes common
// to every data source schema.
func identityDataSourceAttributes(nameDesc string) map[string]dsschema.Attribute {
	return map[string]dsschema.Attribute{
		attrID:        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Synthetic identifier `namespace/name`."},
		attrName:      dsschema.StringAttribute{Required: true, MarkdownDescription: nameDesc},
		attrNamespace: dsschema.StringAttribute{Required: true, MarkdownDescription: "Tenant namespace."},
	}
}

// statusDataSourceAttributes returns the computed ready/chart_version attributes.
func statusDataSourceAttributes() map[string]dsschema.Attribute {
	return map[string]dsschema.Attribute{
		attrReady:        dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the application's `Ready` condition is true."},
		attrChartVersion: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Deployed chart version."},
	}
}

// resourcesDataSourceAttribute returns the computed resources{cpu,memory} block.
func resourcesDataSourceAttribute() dsschema.SingleNestedAttribute {
	return dsschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Explicit CPU and memory per replica, when set.",
		Attributes: map[string]dsschema.Attribute{
			attrCPU:    dsschema.StringAttribute{Computed: true, MarkdownDescription: "CPU available to each replica."},
			attrMemory: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Memory available to each replica."},
		},
	}
}

// passwordUsersResourceAttribute returns a map of {password} users.
func passwordUsersResourceAttribute(description string) rschema.MapNestedAttribute {
	return rschema.MapNestedAttribute{
		Optional:            true,
		MarkdownDescription: description,
		NestedObject: rschema.NestedAttributeObject{
			Attributes: map[string]rschema.Attribute{
				"password": rschema.StringAttribute{
					Optional:            true,
					Computed:            true,
					Sensitive:           true,
					MarkdownDescription: "Password for the user (autogenerated when omitted).",
				},
			},
		},
	}
}

func passwordUsersDataSourceAttribute(description string) dsschema.MapNestedAttribute {
	return dsschema.MapNestedAttribute{
		Computed:            true,
		MarkdownDescription: description,
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"password": dsschema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "User password."},
			},
		},
	}
}

// rolesMapResourceAttribute returns a map of {roles{admin,readonly}} entries,
// shared by database and vhost configuration.
func rolesMapResourceAttribute(description string) rschema.MapNestedAttribute {
	return rschema.MapNestedAttribute{
		Optional:            true,
		MarkdownDescription: description,
		NestedObject: rschema.NestedAttributeObject{
			Attributes: map[string]rschema.Attribute{
				"roles": rschema.SingleNestedAttribute{
					Optional:            true,
					MarkdownDescription: "User roles for this entry.",
					Attributes: map[string]rschema.Attribute{
						"admin":    rschema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Users with admin privileges."},
						"readonly": rschema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Users with read-only privileges."},
					},
				},
			},
		},
	}
}

func rolesMapDataSourceAttribute(description string) dsschema.MapNestedAttribute {
	return dsschema.MapNestedAttribute{
		Computed:            true,
		MarkdownDescription: description,
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"roles": dsschema.SingleNestedAttribute{
					Computed:            true,
					MarkdownDescription: "User roles for this entry.",
					Attributes: map[string]dsschema.Attribute{
						"admin":    dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Users with admin privileges."},
						"readonly": dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Users with read-only privileges."},
					},
				},
			},
		},
	}
}
