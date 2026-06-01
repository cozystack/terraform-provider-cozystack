package provider

import (
	"maps"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func tcpbalancerSchema() rschema.Schema {
	attributes := identityResourceAttributes("TCPBalancer instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrReplicas: rschema.Int64Attribute{
			Optional: true, Computed: true,
			Default:             int64default.StaticInt64(2),
			MarkdownDescription: "Number of HAProxy replicas.",
		},
		attrResources:       resourcesResourceAttribute(),
		attrResourcesPreset: presetAttribute("t1.nano"),
		attrExternal:        externalAttribute(),
		"whitelist_http": rschema.BoolAttribute{
			Optional: true, Computed: true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Secure HTTP by whitelisting client networks.",
		},
		"whitelist": rschema.ListAttribute{
			Optional:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "List of allowed client networks (CIDRs).",
		},
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed TCP balancer, deployed inside a tenant namespace. " +
			"The httpAndHttps block uses server defaults.",
		Attributes: attributes,
	}
}

func tcpbalancerDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("TCPBalancer instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrReplicas:        dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Number of HAProxy replicas."},
		attrResources:       resourcesDataSourceAttribute(),
		attrResourcesPreset: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
		attrExternal:        dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
		"whitelist_http":    dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether HTTP whitelisting is enabled."},
		"whitelist":         dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Allowed client networks."},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack TCP balancer by name and namespace.",
		Attributes:          attributes,
	}
}
