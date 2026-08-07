package provider

import (
	"context"
	"testing"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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

// specModel is the expand/flatten pair every application model implements. It
// lets the shared-block tests below drive each kind through the same checks.
type specModel interface {
	expand(ctx context.Context) (*client.Application, diag.Diagnostics)
	flatten(app *client.Application) diag.Diagnostics
}

func expandSpec(t *testing.T, model specModel) map[string]any {
	t.Helper()

	app, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	return app.Spec
}

func flattenSpec(t *testing.T, model specModel, spec map[string]any) {
	t.Helper()

	app := &client.Application{Name: "instance", Namespace: "tenant-root", Spec: spec}
	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}
}

// tlsBlock builds a tls block with an explicit enabled flag.
func tlsBlock(enabled bool) types.Object {
	return types.ObjectValueMust(tlsObjectType(), map[string]attr.Value{attrEnabled: types.BoolValue(enabled)})
}

// backupBlock builds a backup block with an explicit use_system_bucket flag.
func backupBlock(useSystemBucket bool) types.Object {
	return types.ObjectValueMust(
		backupObjectType(),
		map[string]attr.Value{attrUseSystemBucket: types.BoolValue(useSystemBucket)},
	)
}

// blockCase wires one kind's model to a shared nested block: expand it with the
// block set to value, and flatten a server spec back to the block.
type blockCase struct {
	expand  func(t *testing.T, value types.Object) map[string]any
	flatten func(t *testing.T, spec map[string]any) types.Object
}

// tlsCases covers every kind carrying the v1.6 tls block.
func tlsCases() map[string]blockCase {
	return map[string]blockCase{
		"kafka": {
			expand: func(t *testing.T, value types.Object) map[string]any {
				t.Helper()

				model := fullKafkaModel()
				model.TLS = value

				return expandSpec(t, &model)
			},
			flatten: func(t *testing.T, spec map[string]any) types.Object {
				t.Helper()

				var model kafkaModel

				flattenSpec(t, &model, spec)

				return model.TLS
			},
		},
		"nats": {
			expand: func(t *testing.T, value types.Object) map[string]any {
				t.Helper()

				model := fullNatsModel()
				model.TLS = value

				return expandSpec(t, &model)
			},
			flatten: func(t *testing.T, spec map[string]any) types.Object {
				t.Helper()

				var model natsModel

				flattenSpec(t, &model, spec)

				return model.TLS
			},
		},
		"qdrant": {
			expand: func(t *testing.T, value types.Object) map[string]any {
				t.Helper()

				model := fullQdrantModel()
				model.TLS = value

				return expandSpec(t, &model)
			},
			flatten: func(t *testing.T, spec map[string]any) types.Object {
				t.Helper()

				var model qdrantModel

				flattenSpec(t, &model, spec)

				return model.TLS
			},
		},
		"postgresql": {
			expand: func(t *testing.T, value types.Object) map[string]any {
				t.Helper()

				model := fullPostgresqlModel()
				model.TLS = value

				return expandSpec(t, &model)
			},
			flatten: func(t *testing.T, spec map[string]any) types.Object {
				t.Helper()

				var model postgresqlModel

				flattenSpec(t, &model, spec)

				return model.TLS
			},
		},
	}
}

// TestTLSBlock_UnsetOmitsKey pins the tri-state contract: an unset block leaves
// the spec key out entirely, so the chart keeps inheriting `external`.
func TestTLSBlock_UnsetOmitsKey(t *testing.T) {
	t.Parallel()

	for name, kind := range tlsCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, ok := kind.expand(t, types.ObjectNull(tlsObjectType()))[specTLS]; ok {
				t.Errorf("null tls emits the %q key, want it omitted", specTLS)
			}

			if _, ok := kind.expand(t, types.ObjectUnknown(tlsObjectType()))[specTLS]; ok {
				t.Errorf("unknown tls emits the %q key, want it omitted", specTLS)
			}

			blockWithoutEnabled := types.ObjectValueMust(
				tlsObjectType(),
				map[string]attr.Value{attrEnabled: types.BoolNull()},
			)
			if _, ok := kind.expand(t, blockWithoutEnabled)[specTLS]; ok {
				t.Errorf("tls without enabled emits the %q key, want it omitted", specTLS)
			}
		})
	}
}

// TestTLSBlock_SetEmitsEnabled checks both explicit states reach the spec — an
// explicit false is the whole point of the override.
func TestTLSBlock_SetEmitsEnabled(t *testing.T) {
	t.Parallel()

	for name, kind := range tlsCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, want := range []bool{true, false} {
				block, ok := kind.expand(t, tlsBlock(want))[specTLS].(map[string]any)
				if !ok {
					t.Fatalf("tls enabled=%v did not emit a %q object", want, specTLS)
				}

				if block[attrEnabled] != want {
					t.Errorf("tls.enabled = %v, want %v", block[attrEnabled], want)
				}
			}
		})
	}
}

// TestTLSBlock_Flatten covers the read direction, including the `tls: {}` form
// the upstream schema default produces, which must stay null rather than drift.
func TestTLSBlock_Flatten(t *testing.T) {
	t.Parallel()

	for name, kind := range tlsCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := kind.flatten(t, map[string]any{}); !got.IsNull() {
				t.Errorf("absent tls flattens to %v, want null", got)
			}

			if got := kind.flatten(t, map[string]any{specTLS: map[string]any{}}); !got.IsNull() {
				t.Errorf("empty tls flattens to %v, want null", got)
			}

			got := kind.flatten(t, map[string]any{specTLS: map[string]any{attrEnabled: false}})
			if got.IsNull() {
				t.Fatalf("tls.enabled=false flattens to null, want an object")
			}

			enabled, _ := got.Attributes()[attrEnabled].(types.Bool)
			if enabled.IsNull() || enabled.ValueBool() {
				t.Errorf("tls.enabled = %v, want false", enabled)
			}
		})
	}
}

// backupCases covers the kinds carrying the minimal backup block.
func backupCases() map[string]blockCase {
	return map[string]blockCase{
		"postgresql": {
			expand: func(t *testing.T, value types.Object) map[string]any {
				t.Helper()

				model := fullPostgresqlModel()
				model.Backup = value

				return expandSpec(t, &model)
			},
			flatten: func(t *testing.T, spec map[string]any) types.Object {
				t.Helper()

				var model postgresqlModel

				flattenSpec(t, &model, spec)

				return model.Backup
			},
		},
		"clickhouse": {
			expand: func(t *testing.T, value types.Object) map[string]any {
				t.Helper()

				model := fullClickhouseModel()
				model.Backup = value

				return expandSpec(t, &model)
			},
			flatten: func(t *testing.T, spec map[string]any) types.Object {
				t.Helper()

				var model clickhouseModel

				flattenSpec(t, &model, spec)

				return model.Backup
			},
		},
	}
}

// TestBackupBlock_UnsetOmitsKey keeps the block presence-preserving: the rest of
// the backup spec is deliberately unmanaged, so an unset block must leave the
// key out rather than pin a partial backup object the chart would then honour.
func TestBackupBlock_UnsetOmitsKey(t *testing.T) {
	t.Parallel()

	for name, kind := range backupCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, ok := kind.expand(t, types.ObjectNull(backupObjectType()))[specBackup]; ok {
				t.Errorf("null backup emits the %q key, want it omitted", specBackup)
			}

			blockWithoutFlag := types.ObjectValueMust(
				backupObjectType(),
				map[string]attr.Value{attrUseSystemBucket: types.BoolNull()},
			)
			if _, ok := kind.expand(t, blockWithoutFlag)[specBackup]; ok {
				t.Errorf("backup without use_system_bucket emits the %q key, want it omitted", specBackup)
			}
		})
	}
}

func TestBackupBlock_RoundTrip(t *testing.T) {
	t.Parallel()

	for name, kind := range backupCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			block, ok := kind.expand(t, backupBlock(true))[specBackup].(map[string]any)
			if !ok {
				t.Fatalf("backup block did not emit a %q object", specBackup)
			}

			if block[specUseSystemBucket] != true {
				t.Errorf("backup.useSystemBucket = %v, want true", block[specUseSystemBucket])
			}

			if got := kind.flatten(t, map[string]any{specBackup: map[string]any{}}); !got.IsNull() {
				t.Errorf("backup without useSystemBucket flattens to %v, want null", got)
			}

			// The shape an untouched instance actually reads back as: the
			// aggregated apiserver materialises the upstream schema defaults,
			// so the flag is present and false even though nobody set it. Only
			// the modelled leaf may reach state.
			served := kind.flatten(t, map[string]any{specBackup: map[string]any{
				"enabled":           false,
				specUseSystemBucket: false,
				"retentionPolicy":   "30d",
			}})
			if served.IsNull() {
				t.Fatalf("server-defaulted backup flattens to null, want an object")
			}

			if len(served.Attributes()) != 1 {
				t.Errorf("backup block carries %v, want only the system-bucket flag", served.Attributes())
			}

			got := kind.flatten(t, map[string]any{specBackup: block})

			flag, _ := got.Attributes()[attrUseSystemBucket].(types.Bool)
			if flag.IsNull() || !flag.ValueBool() {
				t.Errorf("backup.use_system_bucket = %v, want true", flag)
			}
		})
	}
}

// TestBackupBlockIsComputed pins the backup block as computed. The aggregated
// apiserver materialises the upstream schema defaults on every read, and the
// upstream backup schema defaults the system-bucket flag itself, so an instance
// that never asked for backups still reads back as `backup: {useSystemBucket:
// false}`. An optional-only block has nowhere to put that value when the
// practitioner omits the block, and every apply then fails with an
// inconsistent-result error.
func TestBackupBlockIsComputed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	kinds := map[string]bool{"cozystack_postgres": true, "cozystack_clickhouse": true}
	seen := 0

	for _, factory := range New("test")().Resources(ctx) {
		res := factory()

		var metadata resource.MetadataResponse

		res.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "cozystack"}, &metadata)

		if !kinds[metadata.TypeName] {
			continue
		}

		seen++

		var schemaResp resource.SchemaResponse

		res.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

		backup, ok := schemaResp.Schema.Attributes[specBackup]
		if !ok {
			t.Errorf("%s has no backup block", metadata.TypeName)

			continue
		}

		if !backup.IsComputed() {
			t.Errorf("%s: backup block is not computed", metadata.TypeName)
		}
	}

	if seen != len(kinds) {
		t.Errorf("checked %d kinds, want %d", seen, len(kinds))
	}
}
