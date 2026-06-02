package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// rawSpecSchema builds a resource schema for a cluster-scoped kind whose spec is
// surfaced as a single normalized-JSON object.
func rawSpecSchema(schemaDesc, nameDesc, specDesc string) rschema.Schema {
	attributes := clusterIdentityResourceAttributes(nameDesc)

	maps.Copy(attributes, map[string]rschema.Attribute{
		"spec": rschema.StringAttribute{
			Optional:            true,
			Computed:            true,
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: specDesc,
		},
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{MarkdownDescription: schemaDesc, Attributes: attributes}
}

// rawSpecDataSourceSchema builds the matching data source schema.
func rawSpecDataSourceSchema(schemaDesc, nameDesc string) dsschema.Schema {
	attributes := clusterIdentityDataSourceAttributes(nameDesc)

	maps.Copy(attributes, map[string]dsschema.Attribute{
		"spec": dsschema.StringAttribute{
			Computed:            true,
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: "Full object spec as JSON.",
		},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{MarkdownDescription: schemaDesc, Attributes: attributes}
}

func packageSourceSchema() rschema.Schema {
	return rawSpecSchema(
		"A Cozystack PackageSource (cluster-scoped, `cozystack.io` group): defines where "+
			"packages come from and their variants. The spec is a platform document, surfaced as JSON.",
		"PackageSource name (`metadata.name`). Immutable.",
		"Full PackageSource spec as JSON (`sourceRef`, `variants`).",
	)
}

func packageSourceDataSourceSchema() dsschema.Schema {
	return rawSpecDataSourceSchema("Read an existing Cozystack PackageSource by name.", "PackageSource name.")
}

func applicationDefinitionSchema() rschema.Schema {
	return rawSpecSchema(
		"A Cozystack ApplicationDefinition (cluster-scoped, `cozystack.io` group): registers a "+
			"tenant application kind. The spec is a platform document, surfaced as JSON.",
		"ApplicationDefinition name (`metadata.name`). Immutable.",
		"Full ApplicationDefinition spec as JSON (`application`, `release`, selectors, `dashboard`).",
	)
}

func applicationDefinitionDataSourceSchema() dsschema.Schema {
	return rawSpecDataSourceSchema("Read an existing Cozystack ApplicationDefinition by name.", "ApplicationDefinition name.")
}

func schedulingClassSchema() rschema.Schema {
	return rawSpecSchema(
		"A Cozystack SchedulingClass (cluster-scoped, `cozystack.io` group): named placement policy "+
			"(nodeSelector/affinities/topologySpreadConstraints). The spec is surfaced as JSON.",
		"SchedulingClass name (`metadata.name`). Immutable.",
		"Full SchedulingClass spec as JSON (`nodeSelector`, `nodeAffinity`, `podAffinity`, "+
			"`podAntiAffinity`, `topologySpreadConstraints`).",
	)
}

func schedulingClassDataSourceSchema() dsschema.Schema {
	return rawSpecDataSourceSchema("Read an existing Cozystack SchedulingClass by name.", "SchedulingClass name.")
}
