package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// rawSpecModel is the shared model for cluster-scoped cozystack.io kinds whose
// spec is a deeply-nested platform document (PackageSource, ApplicationDefinition,
// SchedulingClass). Rather than enumerate dozens of nested, version-coupled
// fields, the whole spec is carried as a normalized JSON object, fully managed
// and diffed semantically.
type rawSpecModel struct {
	ID           types.String         `tfsdk:"id"`
	Name         types.String         `tfsdk:"name"`
	Spec         jsontypes.Normalized `tfsdk:"spec"`
	Ready        types.Bool           `tfsdk:"ready"`
	UID          types.String         `tfsdk:"uid"`
	ChartVersion types.String         `tfsdk:"chart_version"`
}

type rawSpecResourceModel struct {
	rawSpecModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *rawSpecResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

// identity returns an empty namespace (cluster-scoped) and the object name.
func (m *rawSpecModel) identity() (string, string) {
	return "", m.Name.ValueString()
}

func (m *rawSpecModel) expand(_ context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	spec := map[string]any{}

	if !m.Spec.IsNull() && !m.Spec.IsUnknown() {
		if err := json.Unmarshal([]byte(m.Spec.ValueString()), &spec); err != nil {
			diags.AddError("Invalid spec JSON", err.Error())

			return nil, diags
		}
	}

	return &client.Application{Name: m.Name.ValueString(), Spec: spec}, diags
}

func (m *rawSpecModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Name)
	m.Name = types.StringValue(app.Name)
	m.Spec = flattenJSONMap(app.Spec)
	m.Ready = types.BoolValue(app.Status.Ready)
	m.UID = types.StringValue(app.UID)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}
