package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// httpcacheModel maps the cozystack_httpcache schema to Go types. The haproxy and
// nginx tuning blocks are not managed (they use server defaults).
type httpcacheModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Size         types.String `tfsdk:"size"`
	StorageClass types.String `tfsdk:"storage_class"`
	External     types.Bool   `tfsdk:"external"`
	Endpoints    types.List   `tfsdk:"endpoints"`
	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
	UID          types.String `tfsdk:"uid"`
}

type httpcacheResourceModel struct {
	httpcacheModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *httpcacheResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *httpcacheModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func (m *httpcacheModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	endpoints, eDiags := expandStringList(ctx, m.Endpoints)
	diags.Append(eDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrSize:         m.Size.ValueString(),
		specStorageClass: m.StorageClass.ValueString(),
		attrExternal:     m.External.ValueBool(),
		"endpoints":      endpoints,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func (m *httpcacheModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Size = types.StringValue(specString(app.Spec, attrSize))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.External = types.BoolValue(specBool(app.Spec, attrExternal))

	endpoints, eDiags := flattenStringList(app.Spec["endpoints"])
	diags.Append(eDiags...)

	m.Endpoints = endpoints

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}
