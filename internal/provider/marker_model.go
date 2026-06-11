package provider

import (
	"context"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// markerModel is the model for a cluster-scoped marker kind that has no spec:
// its existence (by name) is the entire configuration (e.g. TenantNamespace).
type markerModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Ready        types.Bool   `tfsdk:"ready"`
	UID          types.String `tfsdk:"uid"`
	ChartVersion types.String `tfsdk:"chart_version"`
}

type markerResourceModel struct {
	markerModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *markerResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *markerModel) identity() (string, string) {
	return "", m.Name.ValueString()
}

func (m *markerModel) expand(_ context.Context) (*client.Application, diag.Diagnostics) {
	return &client.Application{Name: m.Name.ValueString()}, nil
}

func (m *markerModel) flatten(app *client.Application) diag.Diagnostics {
	m.ID = types.StringValue(app.Name)
	m.Name = types.StringValue(app.Name)
	m.Ready = types.BoolValue(app.Status.Ready)
	m.UID = types.StringValue(app.UID)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return nil
}

// markerNsModel is the namespaced counterpart (e.g. TenantModule).
type markerNsModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Ready        types.Bool   `tfsdk:"ready"`
	UID          types.String `tfsdk:"uid"`
	ChartVersion types.String `tfsdk:"chart_version"`
}

type markerNsResourceModel struct {
	markerNsModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *markerNsResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *markerNsModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func (m *markerNsModel) expand(_ context.Context) (*client.Application, diag.Diagnostics) {
	return &client.Application{Name: m.Name.ValueString(), Namespace: m.Namespace.ValueString()}, nil
}

func (m *markerNsModel) flatten(app *client.Application) diag.Diagnostics {
	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Ready = types.BoolValue(app.Status.Ready)
	m.UID = types.StringValue(app.UID)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return nil
}
