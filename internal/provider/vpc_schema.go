package provider

import (
	"maps"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func vpcSubnetsResourceAttribute() rschema.ListNestedAttribute {
	return rschema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Subnets of the VPC.",
		NestedObject: rschema.NestedAttributeObject{
			Attributes: map[string]rschema.Attribute{
				"name": rschema.StringAttribute{Required: true, MarkdownDescription: "Subnet name."},
				"cidr": rschema.StringAttribute{Optional: true, MarkdownDescription: "IP address range (CIDR)."},
			},
		},
	}
}

func vpcPeersResourceAttribute() rschema.ListNestedAttribute {
	return rschema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: "VPC peering connections (bidirectional declaration required).",
		NestedObject: rschema.NestedAttributeObject{
			Attributes: map[string]rschema.Attribute{
				"tenant_namespace": rschema.StringAttribute{Required: true, MarkdownDescription: "Namespace of the remote tenant."},
				"vpc_name":         rschema.StringAttribute{Required: true, MarkdownDescription: "Logical name of the remote VPC."},
			},
		},
	}
}

func vpcRoutesResourceAttribute() rschema.ListNestedAttribute {
	return rschema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Static routes for the VPC.",
		NestedObject: rschema.NestedAttributeObject{
			Attributes: map[string]rschema.Attribute{
				"cidr":        rschema.StringAttribute{Required: true, MarkdownDescription: "Destination CIDR."},
				"next_hop_ip": rschema.StringAttribute{Required: true, MarkdownDescription: "Next hop IP address."},
			},
		},
	}
}

func vpcSchema() rschema.Schema {
	attributes := identityResourceAttributes("VirtualPrivateCloud instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		"subnets": vpcSubnetsResourceAttribute(),
		"peers":   vpcPeersResourceAttribute(),
		"routes":  vpcRoutesResourceAttribute(),
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack virtual private cloud (VPC), deployed inside a tenant namespace.",
		Attributes:          attributes,
	}
}

func vpcSubnetsDataSourceAttribute() dsschema.ListNestedAttribute {
	return dsschema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Subnets of the VPC.",
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Subnet name."},
				"cidr": dsschema.StringAttribute{Computed: true, MarkdownDescription: "IP address range (CIDR)."},
			},
		},
	}
}

func vpcPeersDataSourceAttribute() dsschema.ListNestedAttribute {
	return dsschema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: "VPC peering connections.",
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"tenant_namespace": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Namespace of the remote tenant."},
				"vpc_name":         dsschema.StringAttribute{Computed: true, MarkdownDescription: "Logical name of the remote VPC."},
			},
		},
	}
}

func vpcRoutesDataSourceAttribute() dsschema.ListNestedAttribute {
	return dsschema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Static routes for the VPC.",
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"cidr":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Destination CIDR."},
				"next_hop_ip": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Next hop IP address."},
			},
		},
	}
}

func vpcDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("VirtualPrivateCloud instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		"subnets": vpcSubnetsDataSourceAttribute(),
		"peers":   vpcPeersDataSourceAttribute(),
		"routes":  vpcRoutesDataSourceAttribute(),
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack VPC by name and namespace.",
		Attributes:          attributes,
	}
}
