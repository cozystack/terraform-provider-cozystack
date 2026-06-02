package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// packageModel maps the cozystack_package schema to Go types. Package is a
// cluster-scoped resource in the cozystack.io platform group, so it has no
// namespace. Per-component overrides are carried as normalized JSON.
type packageModel struct {
	ID                 types.String         `tfsdk:"id"`
	Name               types.String         `tfsdk:"name"`
	Variant            types.String         `tfsdk:"variant"`
	IgnoreDependencies types.List           `tfsdk:"ignore_dependencies"`
	Components         jsontypes.Normalized `tfsdk:"components"`
	Ready              types.Bool           `tfsdk:"ready"`
	UID                types.String         `tfsdk:"uid"`
	ChartVersion       types.String         `tfsdk:"chart_version"`
}

type packageResourceModel struct {
	packageModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *packageResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

// identity returns an empty namespace (cluster-scoped) and the package name.
func (m *packageModel) identity() (string, string) {
	return "", m.Name.ValueString()
}

func (m *packageModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	ignore, iDiags := expandStringList(ctx, m.IgnoreDependencies)
	diags.Append(iDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{"variant": m.Variant.ValueString()}

	if len(ignore) > 0 {
		spec["ignoreDependencies"] = ignore
	}

	if !m.Components.IsNull() && !m.Components.IsUnknown() {
		var components map[string]any
		if err := json.Unmarshal([]byte(m.Components.ValueString()), &components); err != nil {
			diags.AddError("Invalid components JSON", err.Error())

			return nil, diags
		}

		spec["components"] = components
	}

	return &client.Application{Name: m.Name.ValueString(), Spec: spec}, diags
}

func (m *packageModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Name)
	m.Name = types.StringValue(app.Name)
	m.Variant = types.StringValue(specString(app.Spec, "variant"))

	ignore, iDiags := flattenStringList(app.Spec["ignoreDependencies"])
	diags.Append(iDiags...)

	m.IgnoreDependencies = ignore
	m.Components = flattenJSONMap(app.Spec["components"])

	m.Ready = types.BoolValue(app.Status.Ready)
	m.UID = types.StringValue(app.UID)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}

// flattenJSONMap renders a spec submap as normalized JSON, or null when absent.
func flattenJSONMap(raw any) jsontypes.Normalized {
	value, ok := raw.(map[string]any)
	if !ok || len(value) == 0 {
		return jsontypes.NewNormalizedNull()
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return jsontypes.NewNormalizedNull()
	}

	return jsontypes.NewNormalizedValue(string(encoded))
}
