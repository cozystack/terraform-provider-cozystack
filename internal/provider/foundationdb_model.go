package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// foundationdbModel maps the cozystack_foundationdb schema to Go types. The
// cluster, deprecated backup, monitoring, and securityContext blocks are not
// managed (they use server defaults).
type foundationdbModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Namespace             types.String `tfsdk:"namespace"`
	Storage               types.Object `tfsdk:"storage"`
	Resources             types.Object `tfsdk:"resources"`
	ResourcesPreset       types.String `tfsdk:"resources_preset"`
	CustomParameters      types.List   `tfsdk:"custom_parameters"`
	ImageType             types.String `tfsdk:"image_type"`
	AutomaticReplacements types.Bool   `tfsdk:"automatic_replacements"`
	Ready                 types.Bool   `tfsdk:"ready"`
	ChartVersion          types.String `tfsdk:"chart_version"`
	UID                   types.String `tfsdk:"uid"`
}

type foundationdbResourceModel struct {
	foundationdbModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *foundationdbResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *foundationdbModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func fdbStorageObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"size":          types.StringType,
		"storage_class": types.StringType,
	}
}

func fdbStorageDefault() types.Object {
	return types.ObjectValueMust(fdbStorageObjectType(), map[string]attr.Value{
		"size":          types.StringValue("16Gi"),
		"storage_class": types.StringValue(""),
	})
}

type fdbStorageData struct {
	Size         types.String `tfsdk:"size"`
	StorageClass types.String `tfsdk:"storage_class"`
}

func (m *foundationdbModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	storage, sDiags := expandFdbStorage(ctx, m.Storage)
	diags.Append(sDiags...)

	resources, rDiags := expandResources(ctx, m.Resources)
	diags.Append(rDiags...)

	params, pDiags := expandStringList(ctx, m.CustomParameters)
	diags.Append(pDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		"storage":               storage,
		attrResources:           resources,
		specResourcesPreset:     m.ResourcesPreset.ValueString(),
		"customParameters":      params,
		"imageType":             m.ImageType.ValueString(),
		"automaticReplacements": m.AutomaticReplacements.ValueBool(),
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func expandFdbStorage(ctx context.Context, value types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	var data fdbStorageData

	diags.Append(value.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return out, diags
	}

	out["size"] = data.Size.ValueString()
	out[specStorageClass] = data.StorageClass.ValueString()

	return out, diags
}

func (m *foundationdbModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.ResourcesPreset = types.StringValue(specString(app.Spec, specResourcesPreset))
	m.ImageType = types.StringValue(specString(app.Spec, "imageType"))
	m.AutomaticReplacements = types.BoolValue(specBool(app.Spec, "automaticReplacements"))

	m.Storage = flattenFdbStorage(app.Spec["storage"])

	resources, rDiags := flattenResources(app.Spec[attrResources])
	diags.Append(rDiags...)

	m.Resources = resources

	params, pDiags := flattenStringList(app.Spec["customParameters"])
	diags.Append(pDiags...)

	m.CustomParameters = params

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}

func flattenFdbStorage(raw any) types.Object {
	storage, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(fdbStorageObjectType())
	}

	return types.ObjectValueMust(fdbStorageObjectType(), map[string]attr.Value{
		"size":          types.StringValue(specString(storage, "size")),
		"storage_class": types.StringValue(specString(storage, specStorageClass)),
	})
}
