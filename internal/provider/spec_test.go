package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// kernelModuleObjectType stands in for a nested-object spec entry whose empty
// form is meaningful, exercising the presence-preserving helpers end to end.
func kernelModuleObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		attrName:     types.StringType,
		"parameters": types.ListType{ElemType: types.StringType},
	}
}

type kernelModuleData struct {
	Name       types.String `tfsdk:"name"`
	Parameters types.List   `tfsdk:"parameters"`
}

func buildKernelModule(module kernelModuleData) map[string]any {
	entry := map[string]any{attrName: module.Name.ValueString()}

	_ = setOptionalStringList(context.Background(), entry, "parameters", module.Parameters)

	return entry
}

func kernelModuleList(t *testing.T, modules ...attr.Value) types.List {
	t.Helper()

	return types.ListValueMust(types.ObjectType{AttrTypes: kernelModuleObjectType()}, modules)
}

func TestSetOptionalObjectList_NullOmitsKey(t *testing.T) {
	t.Parallel()

	spec := map[string]any{}

	diags := setOptionalObjectList(
		context.Background(),
		spec,
		"kernelModules",
		types.ListNull(types.ObjectType{AttrTypes: kernelModuleObjectType()}),
		buildKernelModule,
	)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if _, ok := spec["kernelModules"]; ok {
		t.Errorf("kernelModules present for a null attribute, want the key omitted")
	}
}

func TestSetOptionalObjectList_UnknownOmitsKey(t *testing.T) {
	t.Parallel()

	spec := map[string]any{}

	diags := setOptionalObjectList(
		context.Background(),
		spec,
		"kernelModules",
		types.ListUnknown(types.ObjectType{AttrTypes: kernelModuleObjectType()}),
		buildKernelModule,
	)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if _, ok := spec["kernelModules"]; ok {
		t.Errorf("kernelModules present for an unknown attribute, want the key omitted")
	}
}

func TestSetOptionalObjectList_EmptyWritesEmptyList(t *testing.T) {
	t.Parallel()

	spec := map[string]any{}

	diags := setOptionalObjectList(
		context.Background(),
		spec,
		"kernelModules",
		kernelModuleList(t),
		buildKernelModule,
	)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	raw, ok := spec["kernelModules"]
	if !ok {
		t.Fatalf("kernelModules absent for an empty list, want an explicit empty list")
	}

	items, ok := raw.([]any)
	if !ok || len(items) != 0 {
		t.Errorf("kernelModules = %#v, want an empty []any", raw)
	}
}

func TestSetOptionalObjectList_PopulatedWritesEntries(t *testing.T) {
	t.Parallel()

	module := types.ObjectValueMust(kernelModuleObjectType(), map[string]attr.Value{
		attrName:     types.StringValue("nvidia"),
		"parameters": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("NVreg_NvLinkDisable=1")}),
	})

	spec := map[string]any{}

	diags := setOptionalObjectList(
		context.Background(),
		spec,
		"kernelModules",
		kernelModuleList(t, module),
		buildKernelModule,
	)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	items, _ := spec["kernelModules"].([]any)
	if len(items) != 1 {
		t.Fatalf("kernelModules has %d entries, want 1", len(items))
	}

	entry, _ := items[0].(map[string]any)
	if entry[attrName] != "nvidia" {
		t.Errorf("name = %v, want nvidia", entry[attrName])
	}

	parameters, _ := entry["parameters"].([]any)
	if len(parameters) != 1 || parameters[0] != "NVreg_NvLinkDisable=1" {
		t.Errorf("parameters = %v, want [NVreg_NvLinkDisable=1]", parameters)
	}
}

func TestSpecObjectListOrNull_AbsentKeyIsNull(t *testing.T) {
	t.Parallel()

	spec := map[string]any{}

	got, diags := specObjectListOrNull(spec["kernelModules"], kernelModuleObjectType(), flattenKernelModule)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if !got.IsNull() {
		t.Errorf("kernelModules = %v for an absent key, want null", got)
	}
}

func TestSpecObjectListOrNull_EmptyListStaysEmpty(t *testing.T) {
	t.Parallel()

	spec := map[string]any{"kernelModules": []any{}}

	got, diags := specObjectListOrNull(spec["kernelModules"], kernelModuleObjectType(), flattenKernelModule)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if got.IsNull() {
		t.Fatalf("kernelModules is null for a stored empty list, want an empty list")
	}

	if len(got.Elements()) != 0 {
		t.Errorf("kernelModules has %d elements, want 0", len(got.Elements()))
	}
}

func TestSpecObjectListOrNull_RoundTripsEntries(t *testing.T) {
	t.Parallel()

	spec := map[string]any{"kernelModules": []any{
		map[string]any{attrName: "nvidia", "parameters": []any{"NVreg_NvLinkDisable=1"}},
		map[string]any{attrName: "nvidia_uvm"},
	}}

	got, diags := specObjectListOrNull(spec["kernelModules"], kernelModuleObjectType(), flattenKernelModule)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	elements := got.Elements()
	if len(elements) != 2 {
		t.Fatalf("kernelModules has %d elements, want 2", len(elements))
	}

	var modules []kernelModuleData
	if diags := got.ElementsAs(context.Background(), &modules, false); diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if modules[0].Name.ValueString() != "nvidia" || modules[0].Parameters.IsNull() {
		t.Errorf("first module = %+v, want nvidia with parameters", modules[0])
	}

	// A module carrying no parameters keeps the attribute null rather than
	// materialising an empty list the config never wrote.
	if !modules[1].Parameters.IsNull() {
		t.Errorf("second module parameters = %v, want null", modules[1].Parameters)
	}
}

func flattenKernelModule(entry map[string]any) map[string]attr.Value {
	return map[string]attr.Value{
		attrName:     types.StringValue(specString(entry, attrName)),
		"parameters": specStringListOrNull(entry["parameters"]),
	}
}

func TestSetOptionalStringList_ThreeStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   types.List
		present bool
		length  int
	}{
		{name: "null omits the key", value: types.ListNull(types.StringType)},
		{name: "unknown omits the key", value: types.ListUnknown(types.StringType)},
		{
			name:    "empty writes an empty list",
			value:   types.ListValueMust(types.StringType, []attr.Value{}),
			present: true,
		},
		{
			name:    "populated writes the entries",
			value:   types.ListValueMust(types.StringType, []attr.Value{types.StringValue("quiet")}),
			present: true,
			length:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := map[string]any{}

			if diags := setOptionalStringList(context.Background(), spec, "parameters", tt.value); diags.HasError() {
				t.Fatalf("diagnostics: %v", diags)
			}

			raw, ok := spec["parameters"]
			if ok != tt.present {
				t.Fatalf("parameters present = %v, want %v", ok, tt.present)
			}

			if !tt.present {
				return
			}

			items, _ := raw.([]any)
			if len(items) != tt.length {
				t.Errorf("parameters has %d entries, want %d", len(items), tt.length)
			}
		})
	}
}

func TestSpecStringListOrNull_DistinguishesAbsentFromEmpty(t *testing.T) {
	t.Parallel()

	if got := specStringListOrNull(nil); !got.IsNull() {
		t.Errorf("absent key = %v, want null", got)
	}

	got := specStringListOrNull([]any{})
	if got.IsNull() {
		t.Fatalf("stored empty list flattened to null, want an empty list")
	}

	if len(got.Elements()) != 0 {
		t.Errorf("stored empty list has %d elements, want 0", len(got.Elements()))
	}
}

// stringListOrNull is the collapsing counterpart: it is correct for keys whose
// empty and unset forms mean the same thing, and this pins the difference so the
// two helpers do not drift into each other.
func TestStringListOrNull_CollapsesEmptyToNull(t *testing.T) {
	t.Parallel()

	if got := stringListOrNull([]any{}); !got.IsNull() {
		t.Errorf("stored empty list = %v, want null for the collapsing helper", got)
	}
}

func TestSetOptionalString_ThreeStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   types.String
		present bool
		want    string
	}{
		{name: "null omits the key", value: types.StringNull()},
		{name: "unknown omits the key", value: types.StringUnknown()},
		{name: "empty writes an empty string", value: types.StringValue(""), present: true},
		{name: "set writes the value", value: types.StringValue("gpu/nvidia-open"), present: true, want: "gpu/nvidia-open"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := map[string]any{}
			setOptionalString(spec, "schematicID", tt.value)

			raw, ok := spec["schematicID"]
			if ok != tt.present {
				t.Fatalf("schematicID present = %v, want %v", ok, tt.present)
			}

			if tt.present && raw != tt.want {
				t.Errorf("schematicID = %v, want %q", raw, tt.want)
			}
		})
	}
}

func TestSpecStringOrNull_DistinguishesAbsentFromEmpty(t *testing.T) {
	t.Parallel()

	if got := specStringOrNull(map[string]any{}, "schematicID"); !got.IsNull() {
		t.Errorf("absent key = %v, want null", got)
	}

	got := specStringOrNull(map[string]any{"schematicID": ""}, "schematicID")
	if got.IsNull() || got.ValueString() != "" {
		t.Errorf("stored empty string = %v, want an empty string value", got)
	}
}

// optionalSpecString is the collapsing counterpart, reading a stored empty
// string back as null. Pinned so the presence-preserving reader is not swapped
// for it by mistake.
func TestOptionalSpecString_CollapsesEmptyToNull(t *testing.T) {
	t.Parallel()

	if got := optionalSpecString(map[string]any{"schematicID": ""}, "schematicID"); !got.IsNull() {
		t.Errorf("stored empty string = %v, want null for the collapsing helper", got)
	}
}

// jsonList builds a list of JSON documents in the shape the schema declares.
func jsonList(t *testing.T, documents ...string) types.List {
	t.Helper()

	elements := make([]attr.Value, 0, len(documents))
	for _, document := range documents {
		elements = append(elements, jsontypes.NewNormalizedValue(document))
	}

	return types.ListValueMust(jsontypes.NormalizedType{}, elements)
}

func TestSetOptionalJSONList_ThreeStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   types.List
		present bool
		length  int
	}{
		{name: "null omits the key", value: types.ListNull(jsontypes.NormalizedType{})},
		{name: "unknown omits the key", value: types.ListUnknown(jsontypes.NormalizedType{})},
		{name: "empty writes an empty list", value: jsonList(t), present: true},
		{
			name:    "populated writes the documents",
			value:   jsonList(t, `{"name":"auth-config"}`),
			present: true,
			length:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := map[string]any{}

			if diags := setOptionalJSONList(context.Background(), spec, "extraVolumes", tt.value); diags.HasError() {
				t.Fatalf("diagnostics: %v", diags)
			}

			raw, ok := spec["extraVolumes"]
			if ok != tt.present {
				t.Fatalf("extraVolumes present = %v, want %v", ok, tt.present)
			}

			if !tt.present {
				return
			}

			items, _ := raw.([]any)
			if len(items) != tt.length {
				t.Errorf("extraVolumes has %d entries, want %d", len(items), tt.length)
			}
		})
	}
}

// A list element that is not a JSON document is reachable from configuration —
// `extra_volumes = [null]` is enough — so the diagnostic has to say which
// element, not just which key.
func TestSetOptionalJSONList_ReportsTheOffendingElement(t *testing.T) {
	t.Parallel()

	value := types.ListValueMust(jsontypes.NormalizedType{}, []attr.Value{
		jsontypes.NewNormalizedValue(`{"name":"auth-config"}`),
		jsontypes.NewNormalizedNull(),
	})

	spec := map[string]any{}

	diags := setOptionalJSONList(context.Background(), spec, "extraVolumes", value)
	if !diags.HasError() {
		t.Fatalf("a null element produced no error")
	}

	detail := diags.Errors()[0].Detail()
	if !strings.Contains(detail, "Element 1") {
		t.Errorf("detail = %q, want it to name element 1", detail)
	}

	if _, ok := spec["extraVolumes"]; ok {
		t.Errorf("extraVolumes written despite the error")
	}
}

func TestSpecJSONListOrNull_DistinguishesAbsentFromEmpty(t *testing.T) {
	t.Parallel()

	absent, diags := specJSONListOrNull(nil)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if !absent.IsNull() {
		t.Errorf("absent key = %v, want null", absent)
	}

	empty, diags := specJSONListOrNull([]any{})
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if empty.IsNull() || len(empty.Elements()) != 0 {
		t.Errorf("stored empty list = %v, want an empty list", empty)
	}

	got, diags := specJSONListOrNull([]any{map[string]any{"name": "auth-config"}})
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	document, _ := got.Elements()[0].(jsontypes.Normalized)
	if document.ValueString() != `{"name":"auth-config"}` {
		t.Errorf("document = %s, want the auth-config volume", document.ValueString())
	}
}
