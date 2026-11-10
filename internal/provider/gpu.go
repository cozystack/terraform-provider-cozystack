package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"k8s.io/apimachinery/pkg/util/validation"
)

type vmGPUData struct {
	Name      types.String `tfsdk:"name"`
	ClassName types.String `tfsdk:"class_name"`
	Count     types.Int64  `tfsdk:"count"`
}

const gpuDescription = "GPUs to attach. Use either raw resource names or GPUClass names throughout the list, never both. Class entries require GPU step 1 support on the server. The server validates class availability, entitlement and compatibility. NVIDIA drivers need at least 4 GiB RAM."

func vmGPUResourceAttribute() rschema.ListNestedAttribute {
	return rschema.ListNestedAttribute{
		Optional: true, MarkdownDescription: gpuDescription,
		NestedObject: rschema.NestedAttributeObject{Attributes: map[string]rschema.Attribute{
			"name":       rschema.StringAttribute{Optional: true, MarkdownDescription: "Raw device-plugin resource name. Mutually exclusive with class_name; count must be omitted."},
			"class_name": rschema.StringAttribute{Optional: true, MarkdownDescription: "GPUClass name. Mutually exclusive with name."},
			"count":      rschema.Int64Attribute{Optional: true, MarkdownDescription: "Positive number of GPUs from the class, per VM or worker. The server uses one when omitted. Not valid with a raw name."},
		}},
	}
}

func vmGPUDataSourceAttribute() dsschema.ListNestedAttribute {
	return dsschema.ListNestedAttribute{Computed: true, MarkdownDescription: "Configured GPU entries.", NestedObject: dsschema.NestedAttributeObject{Attributes: map[string]dsschema.Attribute{
		"name":       dsschema.StringAttribute{Computed: true, MarkdownDescription: "Raw GPU resource name."},
		"class_name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "GPUClass name."},
		"count":      dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Configured class GPU count, per VM or worker; null when omitted."},
	}}}
}

func expandGPU(data vmGPUData) map[string]any {
	entry := map[string]any{}
	setOptionalString(entry, "name", data.Name)
	setOptionalString(entry, "className", data.ClassName)
	if !data.Count.IsNull() && !data.Count.IsUnknown() {
		entry["count"] = data.Count.ValueInt64()
	}

	return entry
}

func flattenGPU(entry map[string]any) map[string]attr.Value {
	count := types.Int64Null()
	if entry["count"] != nil {
		count = types.Int64Value(specInt64(entry, "count"))
	}

	return map[string]attr.Value{"name": specStringOrNull(entry, "name"), "class_name": specStringOrNull(entry, "className"), "count": count}
}

func validateGPUEntries(list types.List) diag.Diagnostics {
	var diags diag.Diagnostics
	raw, classes := false, false
	var units int64
	for _, value := range list.Elements() {
		object, ok := value.(types.Object)
		if !ok || object.IsUnknown() {
			continue
		}
		if object.IsNull() {
			diags.AddError("Invalid GPU entry", "GPU entries must not be null.")

			continue
		}
		fields := object.Attributes()
		name, _ := fields["name"].(types.String)
		class, _ := fields["class_name"].(types.String)
		count, _ := fields["count"].(types.Int64)
		if name.IsUnknown() || class.IsUnknown() {
			continue
		}
		diags.Append(validateGPUEntry(name, class, count)...)
		if name.ValueString() != "" {
			raw = true

			continue
		}
		if class.ValueString() == "" {
			continue
		}
		classes = true
		units += gpuEntryUnits(count)
	}
	if raw && classes {
		diags.AddError("Mixed GPU entries", "Use only name entries or only class_name entries in a GPU list.")
	}
	if units > 1024 {
		diags.AddError("Too many GPUs", "A VM or worker can request at most 1024 GPU units from class entries.")
	}

	return diags
}

func validateGPUEntry(name, class types.String, count types.Int64) diag.Diagnostics {
	var diags diag.Diagnostics
	if (name.ValueString() == "") == (class.ValueString() == "") {
		diags.AddError("Invalid GPU entry", "Set exactly one nonempty name or class_name.")
	}
	if name.ValueString() != "" && !count.IsNull() && !count.IsUnknown() {
		diags.AddError("Invalid GPU count", "count applies only to class_name entries.")
	}
	if class.ValueString() != "" {
		if errors := validation.IsDNS1123Label(class.ValueString()); len(errors) > 0 {
			diags.AddError("Invalid GPU class_name", strings.Join(errors, "; "))
		}
		if !count.IsNull() && !count.IsUnknown() && (count.ValueInt64() < 1 || count.ValueInt64() > 1024) {
			diags.AddError("Invalid GPU count", "count must be between 1 and 1024.")
		}
	}

	return diags
}

func tenantGPUObjectType() map[string]attr.Type {
	return map[string]attr.Type{"vgpu": types.BoolType}
}

func expandTenantGPU(ctx context.Context, spec map[string]any, object types.Object) diag.Diagnostics {
	var diags diag.Diagnostics
	if object.IsNull() || object.IsUnknown() {
		return diags
	}
	var data struct {
		VGPU types.Bool `tfsdk:"vgpu"`
	}
	diags.Append(object.As(ctx, &data, basetypes.ObjectAsOptions{})...)
	if !diags.HasError() {
		gpu := map[string]any{}
		if !data.VGPU.IsNull() && !data.VGPU.IsUnknown() {
			gpu["vgpu"] = data.VGPU.ValueBool()
		}
		spec["gpu"] = gpu
	}

	return diags
}

func gpuEntryUnits(count types.Int64) int64 {
	if count.IsNull() {
		return 1
	}
	if count.IsUnknown() || count.ValueInt64() < 1 || count.ValueInt64() > 1024 {
		return 0
	}

	return count.ValueInt64()
}
