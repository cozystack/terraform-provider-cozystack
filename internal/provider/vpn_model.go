package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// vpnModel maps the cozystack_vpn schema to Go types. The spec attributes mirror
// the json tags of the pinned vpn.ConfigSpec one-to-one.
type vpnModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Namespace       types.String `tfsdk:"namespace"`
	Replicas        types.Int64  `tfsdk:"replicas"`
	Resources       types.Object `tfsdk:"resources"`
	ResourcesPreset types.String `tfsdk:"resources_preset"`
	External        types.Bool   `tfsdk:"external"`
	Host            types.String `tfsdk:"host"`
	Users           types.Map    `tfsdk:"users"`
	ExternalIPs     types.List   `tfsdk:"external_ips"`
	Ready           types.Bool   `tfsdk:"ready"`
	ChartVersion    types.String `tfsdk:"chart_version"`
	UID             types.String `tfsdk:"uid"`
}

// vpnResourceModel is the resource model with wait behaviour.
type vpnResourceModel struct {
	vpnModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *vpnResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *vpnModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func (m *vpnModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	resources, rDiags := expandResources(ctx, m.Resources)
	diags.Append(rDiags...)

	users, uDiags := expandPasswordUsers(ctx, m.Users)
	diags.Append(uDiags...)

	externalIPs, ipDiags := expandStringList(ctx, m.ExternalIPs)
	diags.Append(ipDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrReplicas:        m.Replicas.ValueInt64(),
		attrResources:       resources,
		specResourcesPreset: m.ResourcesPreset.ValueString(),
		attrExternal:        m.External.ValueBool(),
		attrHost:            m.Host.ValueString(),
		"users":             users,
		"externalIPs":       externalIPs,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func (m *vpnModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Replicas = types.Int64Value(specInt64(app.Spec, attrReplicas))
	m.ResourcesPreset = types.StringValue(specString(app.Spec, specResourcesPreset))
	m.External = types.BoolValue(specBool(app.Spec, attrExternal))
	m.Host = types.StringValue(specString(app.Spec, attrHost))

	resources, rDiags := flattenResources(app.Spec[attrResources])
	diags.Append(rDiags...)

	m.Resources = resources

	users, uDiags := flattenPasswordUsers(app.Spec["users"])
	diags.Append(uDiags...)

	m.Users = users

	externalIPs, ipDiags := flattenStringList(app.Spec["externalIPs"])
	diags.Append(ipDiags...)

	m.ExternalIPs = externalIPs

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}
