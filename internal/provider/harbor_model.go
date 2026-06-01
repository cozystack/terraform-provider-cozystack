package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// harborModel maps the cozystack_harbor schema to Go types. The core, registry,
// jobservice, trivy, database, and redis component blocks are not managed (they
// use server defaults).
type harborModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Host         types.String `tfsdk:"host"`
	StorageClass types.String `tfsdk:"storage_class"`
	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
}

type harborResourceModel struct {
	harborModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *harborResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *harborModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func (m *harborModel) expand(_ context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	spec := map[string]any{
		specStorageClass: m.StorageClass.ValueString(),
	}

	// host is server-defaulted to a tenant subdomain; only send it when set so
	// the computed default does not produce a perpetual diff.
	if host := m.Host.ValueString(); host != "" {
		spec[attrHost] = host
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func (m *harborModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Host = types.StringValue(specString(app.Spec, attrHost))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}
