package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// vpcModel maps the cozystack_vpc schema to Go types. It manages the subnet,
// peering, and static-route lists of a VirtualPrivateCloud.
type vpcModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Subnets      types.List   `tfsdk:"subnets"`
	Peers        types.List   `tfsdk:"peers"`
	Routes       types.List   `tfsdk:"routes"`
	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
}

type vpcResourceModel struct {
	vpcModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *vpcResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *vpcModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func vpcSubnetObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
		"cidr": types.StringType,
	}
}

func vpcPeerObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"tenant_namespace": types.StringType,
		"vpc_name":         types.StringType,
	}
}

func vpcRouteObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"cidr":        types.StringType,
		"next_hop_ip": types.StringType,
	}
}

type vpcSubnetData struct {
	Name types.String `tfsdk:"name"`
	Cidr types.String `tfsdk:"cidr"`
}

type vpcPeerData struct {
	TenantNamespace types.String `tfsdk:"tenant_namespace"`
	VpcName         types.String `tfsdk:"vpc_name"`
}

type vpcRouteData struct {
	Cidr      types.String `tfsdk:"cidr"`
	NextHopIP types.String `tfsdk:"next_hop_ip"`
}

func (m *vpcModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	subnets, sDiags := expandObjectList(ctx, m.Subnets, func(s vpcSubnetData) map[string]any {
		return map[string]any{"name": s.Name.ValueString(), "cidr": s.Cidr.ValueString()}
	})
	diags.Append(sDiags...)

	peers, pDiags := expandObjectList(ctx, m.Peers, func(p vpcPeerData) map[string]any {
		return map[string]any{"tenantNamespace": p.TenantNamespace.ValueString(), "vpcName": p.VpcName.ValueString()}
	})
	diags.Append(pDiags...)

	routes, rDiags := expandObjectList(ctx, m.Routes, func(r vpcRouteData) map[string]any {
		return map[string]any{"cidr": r.Cidr.ValueString(), "nextHopIP": r.NextHopIP.ValueString()}
	})
	diags.Append(rDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		"subnets": subnets,
		"peers":   peers,
		"routes":  routes,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func (m *vpcModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)

	subnets, sDiags := flattenObjectList(app.Spec["subnets"], vpcSubnetObjectType(), func(s map[string]any) map[string]attr.Value {
		name, _ := s["name"].(string)
		cidr, _ := s["cidr"].(string)

		return map[string]attr.Value{"name": types.StringValue(name), "cidr": types.StringValue(cidr)}
	})
	diags.Append(sDiags...)

	m.Subnets = subnets

	peers, pDiags := flattenObjectList(app.Spec["peers"], vpcPeerObjectType(), func(p map[string]any) map[string]attr.Value {
		ns, _ := p["tenantNamespace"].(string)
		vpcName, _ := p["vpcName"].(string)

		return map[string]attr.Value{"tenant_namespace": types.StringValue(ns), "vpc_name": types.StringValue(vpcName)}
	})
	diags.Append(pDiags...)

	m.Peers = peers

	routes, rDiags := flattenObjectList(app.Spec["routes"], vpcRouteObjectType(), func(r map[string]any) map[string]attr.Value {
		cidr, _ := r["cidr"].(string)
		nextHop, _ := r["nextHopIP"].(string)

		return map[string]attr.Value{"cidr": types.StringValue(cidr), "next_hop_ip": types.StringValue(nextHop)}
	})
	diags.Append(rDiags...)

	m.Routes = routes

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}
