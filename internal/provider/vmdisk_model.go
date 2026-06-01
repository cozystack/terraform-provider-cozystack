package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// vmdiskModel maps the cozystack_vmdisk schema to Go types. The source block
// supports cloning a disk, downloading over HTTP, or referencing a named image;
// the interactive upload source is not managed.
type vmdiskModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Source       types.Object `tfsdk:"source"`
	Optical      types.Bool   `tfsdk:"optical"`
	Storage      types.String `tfsdk:"storage"`
	StorageClass types.String `tfsdk:"storage_class"`
	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
}

type vmdiskResourceModel struct {
	vmdiskModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *vmdiskResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *vmdiskModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func vmdiskNameObjectType() map[string]attr.Type {
	return map[string]attr.Type{"name": types.StringType}
}

func vmdiskURLObjectType() map[string]attr.Type {
	return map[string]attr.Type{"url": types.StringType}
}

func vmdiskSourceObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"disk":  types.ObjectType{AttrTypes: vmdiskNameObjectType()},
		"http":  types.ObjectType{AttrTypes: vmdiskURLObjectType()},
		"image": types.ObjectType{AttrTypes: vmdiskNameObjectType()},
	}
}

type vmdiskNameData struct {
	Name types.String `tfsdk:"name"`
}

type vmdiskURLData struct {
	URL types.String `tfsdk:"url"`
}

type vmdiskSourceData struct {
	Disk  *vmdiskNameData `tfsdk:"disk"`
	HTTP  *vmdiskURLData  `tfsdk:"http"`
	Image *vmdiskNameData `tfsdk:"image"`
}

func (m *vmdiskModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	source, sDiags := expandVMDiskSource(ctx, m.Source)
	diags.Append(sDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		"source":         source,
		"optical":        m.Optical.ValueBool(),
		"storage":        m.Storage.ValueString(),
		specStorageClass: m.StorageClass.ValueString(),
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func expandVMDiskSource(ctx context.Context, value types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	var data vmdiskSourceData

	diags.Append(value.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return out, diags
	}

	if data.Disk != nil {
		out["disk"] = map[string]any{"name": data.Disk.Name.ValueString()}
	}

	if data.HTTP != nil {
		out["http"] = map[string]any{"url": data.HTTP.URL.ValueString()}
	}

	if data.Image != nil {
		out["image"] = map[string]any{"name": data.Image.Name.ValueString()}
	}

	return out, diags
}

func (m *vmdiskModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Optical = types.BoolValue(specBool(app.Spec, "optical"))
	m.Storage = types.StringValue(specString(app.Spec, "storage"))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.Source = flattenVMDiskSource(app.Spec["source"])

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}

func flattenVMDiskSource(raw any) types.Object {
	source, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(vmdiskSourceObjectType())
	}

	return types.ObjectValueMust(vmdiskSourceObjectType(), map[string]attr.Value{
		"disk":  vmdiskNameValue(source["disk"]),
		"http":  vmdiskURLValue(source["http"]),
		"image": vmdiskNameValue(source["image"]),
	})
}

func vmdiskNameValue(raw any) attr.Value {
	entry, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(vmdiskNameObjectType())
	}

	name, _ := entry["name"].(string)

	return types.ObjectValueMust(vmdiskNameObjectType(), map[string]attr.Value{"name": types.StringValue(name)})
}

func vmdiskURLValue(raw any) attr.Value {
	entry, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(vmdiskURLObjectType())
	}

	url, _ := entry["url"].(string)

	return types.ObjectValueMust(vmdiskURLObjectType(), map[string]attr.Value{"url": types.StringValue(url)})
}
