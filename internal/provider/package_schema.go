package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func packageSchema() rschema.Schema {
	attributes := clusterIdentityResourceAttributes("Package name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		"variant": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("default"),
			MarkdownDescription: "Variant to use from the PackageSource.",
		},
		"ignore_dependencies": rschema.ListAttribute{
			Optional:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "Package source dependencies to skip installing.",
		},
		"components": rschema.StringAttribute{
			Optional:   true,
			CustomType: jsontypes.NormalizedType{},
			MarkdownDescription: "Per-component overrides as a JSON object keyed by release name " +
				"(`{\"<release>\": {\"enabled\": false, \"values\": {…}}}`).",
		},
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack platform Package (cluster-scoped, `cozystack.io` group): " +
			"installs a package variant from a PackageSource into HelmReleases.",
		Attributes: attributes,
	}
}

func packageDataSourceSchema() dsschema.Schema {
	attributes := clusterIdentityDataSourceAttributes("Package name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		"variant":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "Variant in use."},
		"ignore_dependencies": dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Skipped dependencies."},
		"components":          dsschema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Per-component overrides as JSON."},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack Package by name.",
		Attributes:          attributes,
	}
}
