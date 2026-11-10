package provider

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/kubernetesnodes"
	"github.com/cozystack/cozystack/api/apps/v1alpha1/vminstance"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestGPUStepOneSchemas(t *testing.T) {
	t.Parallel()
	for name, schema := range map[string]rschema.Schema{"vm": vminstanceSchema(), "pool": kubernetesNodesSchema()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gpu := schema.Attributes["gpus"].(rschema.ListNestedAttribute)
			for _, key := range []string{"name", "class_name", "count"} {
				attribute, present := gpu.NestedObject.Attributes[key]
				if !present || !attribute.IsOptional() || attribute.IsComputed() {
					t.Errorf("%s must be optional, non-computed", key)
				}
			}
			if gpu.NestedObject.Attributes["count"].(rschema.Int64Attribute).Default != nil {
				t.Error("count must not default to one on raw entries")
			}
		})
	}
	gpu := tenantSchema().Attributes["gpu"].(rschema.SingleNestedAttribute)
	vgpu := gpu.Attributes["vgpu"].(rschema.BoolAttribute)
	if !gpu.Optional || gpu.Computed || !vgpu.Optional || vgpu.Computed || vgpu.Default != nil {
		t.Error("tenant GPU entitlement must be optional without a provider default")
	}
}

func gpuTestObject(name, class types.String, count types.Int64) types.Object {
	return types.ObjectValueMust(vmGpuObjectType(), map[string]attr.Value{"name": name, "class_name": class, "count": count})
}

func gpuTestList(entries ...attr.Value) types.List {
	return types.ListValueMust(types.ObjectType{AttrTypes: vmGpuObjectType()}, entries)
}

func TestGPUEntriesRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input types.List
		wire  any
	}{
		{"raw-empty-class", gpuTestList(gpuTestObject(types.StringValue("nvidia.com/example"), types.StringValue(""), types.Int64Null())), []any{map[string]any{"name": "nvidia.com/example", "className": ""}}},
		{"class-empty-name", gpuTestList(gpuTestObject(types.StringValue(""), types.StringValue("example"), types.Int64Null())), []any{map[string]any{"name": "", "className": "example"}}},
		{"raw", gpuTestList(gpuTestObject(types.StringValue("nvidia.com/example"), types.StringNull(), types.Int64Null())), []any{map[string]any{"name": "nvidia.com/example"}}},
		{"class-default", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Null())), []any{map[string]any{"className": "example"}}},
		{"class-one", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Value(1))), []any{map[string]any{"className": "example", "count": int64(1)}}},
		{"class-many", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Value(3))), []any{map[string]any{"className": "example", "count": int64(3)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vm := fullVMInstanceModel()
			vm.Gpus = tt.input
			pool := unsetKubernetesNodesModel()
			pool.Gpus = tt.input
			vmApp, diagnostics := vm.expand(context.Background())
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			poolApp, diagnostics := pool.expand(context.Background())
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			for name, app := range map[string]*client.Application{"vm": vmApp, "pool": poolApp} {
				if !reflect.DeepEqual(app.Spec["gpus"], tt.wire) {
					t.Errorf("%s wire %#v, want %#v", name, app.Spec["gpus"], tt.wire)
				}
			}
			if diagnostics := vm.flatten(vmApp); diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if diagnostics := pool.flatten(poolApp); diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if !vm.Gpus.Equal(tt.input) || !pool.Gpus.Equal(tt.input) {
				t.Fatalf("roundtrip vm=%s pool=%s want=%s", vm.Gpus, pool.Gpus, tt.input)
			}
		})
	}
}

func TestGPUEntriesValidation(t *testing.T) {
	t.Parallel()
	raw := gpuTestObject(types.StringValue("nvidia.com/example"), types.StringNull(), types.Int64Null())
	class := gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Null())
	tests := []struct {
		name    string
		list    types.List
		invalid bool
	}{
		{"null-entry", gpuTestList(types.ObjectNull(vmGpuObjectType())), true},
		{"raw", gpuTestList(raw), false},
		{"class", gpuTestList(class), false},
		{"both", gpuTestList(gpuTestObject(types.StringValue("raw"), types.StringValue("example"), types.Int64Null())), true},
		{"neither", gpuTestList(gpuTestObject(types.StringNull(), types.StringNull(), types.Int64Null())), true},
		{"empty-names", gpuTestList(gpuTestObject(types.StringValue(""), types.StringValue(""), types.Int64Null())), true},
		{"mixed", gpuTestList(raw, class), true},
		{"raw-count", gpuTestList(gpuTestObject(types.StringValue("raw"), types.StringNull(), types.Int64Value(1))), true},
		{"zero", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Value(0))), true},
		{"negative", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Value(-1))), true},
		{"limit", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Value(1024))), false},
		{"too-many", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Value(1025))), true},
		{"sum", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Value(1024)), class), true},
		{"bad-class", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("bad/class"), types.Int64Null())), true},
		{"unknown-list", types.ListUnknown(types.ObjectType{AttrTypes: vmGpuObjectType()}), false},
		{"unknown-object", gpuTestList(types.ObjectUnknown(vmGpuObjectType())), false},
		{"unknown-class", gpuTestList(gpuTestObject(types.StringNull(), types.StringUnknown(), types.Int64Null())), false},
		{"unknown-name", gpuTestList(gpuTestObject(types.StringUnknown(), types.StringNull(), types.Int64Null())), false},
		{"unknown-count", gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Unknown())), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := validateGPUEntries(tt.list).HasError(); got != tt.invalid {
				t.Fatalf("invalid=%v want=%v", got, tt.invalid)
			}
		})
	}
}

func TestTenantGPUPresence(t *testing.T) {
	t.Parallel()
	for _, value := range []types.Bool{types.BoolNull(), types.BoolValue(false), types.BoolValue(true)} {
		t.Run(value.String(), func(t *testing.T) {
			t.Parallel()
			model := fullModel()
			model.GPU = types.ObjectValueMust(tenantGPUObjectType(), map[string]attr.Value{"vgpu": value})
			if value.IsNull() {
				model.GPU = types.ObjectNull(tenantGPUObjectType())
			}
			app, diagnostics := model.expand(context.Background())
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if _, present := app.Spec["gpu"]; present == value.IsNull() {
				t.Fatalf("gpu presence=%v for %s", present, value)
			}
			if !value.IsNull() {
				if got := app.Spec["gpu"].(map[string]any)["vgpu"]; got != value.ValueBool() {
					t.Fatalf("vgpu=%v", got)
				}
			}
			resource := tenantResourceModel{tenantModel: model}
			resource.ID = types.StringUnknown()
			app.Spec["gpu"] = map[string]any{"vgpu": value.ValueBool()}
			if diagnostics := resource.flatten(app); diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if !resource.GPU.Equal(model.GPU) {
				t.Fatalf("resource GPU=%s want=%s", resource.GPU, model.GPU)
			}
			var dataSource tenantModel
			if diagnostics := dataSource.flatten(app); diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if got := dataSource.GPU.Attributes()["vgpu"]; !got.Equal(types.BoolValue(value.ValueBool())) {
				t.Fatalf("data source vgpu=%s", got)
			}
		})
	}
}

func TestGPUStepOnePendingSpecKeys(t *testing.T) {
	t.Parallel()
	classes := gpuTestList(gpuTestObject(types.StringNull(), types.StringValue("example"), types.Int64Value(2)))
	vm := fullVMInstanceModel()
	vm.Gpus = classes
	pool := unsetKubernetesNodesModel()
	pool.Gpus = classes
	tenantModel := fullModel()
	tenantModel.GPU = types.ObjectValueMust(tenantGPUObjectType(), map[string]attr.Value{"vgpu": types.BoolValue(true)})
	vmApp, diagnostics := vm.expand(context.Background())
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	poolApp, diagnostics := pool.expand(context.Background())
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	tenantApp, diagnostics := tenantModel.expand(context.Background())
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	for name, app := range map[string]*client.Application{"vm": vmApp, "pool": poolApp} {
		entry := app.Spec["gpus"].([]any)[0].(map[string]any)
		emitted := map[string]bool{}
		for key := range entry {
			emitted[key] = true
		}
		pinned := any(vminstance.GPU{})
		if name == "pool" {
			pinned = kubernetesnodes.GPU{}
		}
		assertGPUEntryCoverage(t, emitted, pinned)

		if _, ok := entry["className"].(string); !ok {
			t.Errorf("%s className has %T", name, entry["className"])
		}
		if _, ok := entry["count"].(int64); !ok {
			t.Errorf("%s count has %T", name, entry["count"])
		}
	}
	gpu := tenantApp.Spec["gpu"].(map[string]any)
	emitted := map[string]bool{"gpu": true}
	assertSpecCoverage(t, emitted, struct {
		GPU struct{} `json:"gpu"`
	}{})
	emitted = map[string]bool{}
	for key := range gpu {
		emitted[key] = true
	}
	assertSpecCoverage(t, emitted, struct {
		VGPU bool `json:"vgpu,omitempty"`
	}{})
	if _, ok := gpu["vgpu"].(bool); !ok {
		t.Errorf("vgpu has %T", gpu["vgpu"])
	}
}

func TestGPUEntriesEmptyAndUnknown(t *testing.T) {
	t.Parallel()
	for _, input := range []types.List{types.ListNull(types.ObjectType{AttrTypes: vmGpuObjectType()}), gpuTestList(), types.ListUnknown(types.ObjectType{AttrTypes: vmGpuObjectType()})} {
		t.Run(input.String(), func(t *testing.T) {
			t.Parallel()
			vm := fullVMInstanceModel()
			vm.Gpus = input
			pool := unsetKubernetesNodesModel()
			pool.Gpus = input
			vmApp, diagnostics := vm.expand(context.Background())
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			poolApp, diagnostics := pool.expand(context.Background())
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			for name, app := range map[string]*client.Application{"vm": vmApp, "pool": poolApp} {
				if _, present := app.Spec["gpus"]; present == (input.IsNull() || input.IsUnknown()) {
					t.Errorf("%s GPU presence=%v for %s", name, present, input)
				}
			}
			if !input.IsUnknown() {
				if diagnostics := vm.flatten(vmApp); diagnostics.HasError() {
					t.Fatal(diagnostics)
				}
				if diagnostics := pool.flatten(poolApp); diagnostics.HasError() {
					t.Fatal(diagnostics)
				}
				if !vm.Gpus.Equal(input) || !pool.Gpus.Equal(input) {
					t.Fatalf("empty/null roundtrip vm=%s pool=%s want=%s", vm.Gpus, pool.Gpus, input)
				}
			}
		})
	}
}

func assertGPUEntryCoverage(t *testing.T, emitted map[string]bool, pinned any) {
	t.Helper()
	fields := maps.Clone(emitted)
	pending := map[string]bool{}
	for _, key := range []string{"className", "count"} {
		if fields[key] {
			pending[key] = true
		}
		delete(fields, key)
	}
	// Remove the split when the pinned API includes the fields from https://github.com/cozystack/cozystack/pull/5030.
	assertSpecCoverage(t, pending, struct {
		ClassName string `json:"className,omitempty"`
		Count     int    `json:"count,omitempty"`
	}{})
	assertSpecCoverage(t, fields, pinned, "name")
}

func TestGPUSpecCoverageRejectsUnknown(t *testing.T) {
	if os.Getenv("TEST_GPU_COVERAGE_HELPER") == "1" {
		assertGPUEntryCoverage(t, map[string]bool{"className": true, "count": true, "classNmae": true}, vminstance.GPU{})
		return
	}
	t.Parallel()
	command := exec.CommandContext(context.Background(), os.Args[0], "--test.run=^TestGPUSpecCoverageRejectsUnknown$")
	command.Env = append(os.Environ(), "TEST_GPU_COVERAGE_HELPER=1")
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), `expand emits "classNmae"`) {
		t.Fatalf("unknown GPU field guard: err=%v output=%s", err, output)
	}
}

func TestGPUResourceConfigValidation(t *testing.T) {
	t.Parallel()
	for _, gpu := range []types.List{gpuTestList(gpuTestObject(types.StringValue("raw"), types.StringValue("example"), types.Int64Null())), gpuTestList(gpuTestObject(types.StringNull(), types.StringUnknown(), types.Int64Unknown()))} {
		t.Run(gpu.String(), func(t *testing.T) {
			t.Parallel()
			vm := vminstanceResourceModel{vminstanceModel: fullVMInstanceModel()}
			vm.Gpus = gpu
			vm.IPAddresses = types.ListNull(types.StringType)
			pool := kubernetesNodesResourceModel{kubernetesNodesModel: unsetKubernetesNodesModel()}
			pool.Gpus = gpu
			for name, fixture := range map[string]struct {
				model  any
				schema rschema.Schema
				res    resource.ResourceWithValidateConfig
			}{
				"vm":   {vm, vminstanceSchema(), newAppResource[vminstanceResourceModel](client.VMInstanceResource(), "vminstance", vminstanceSchema)().(resource.ResourceWithValidateConfig)},
				"pool": {pool, kubernetesNodesSchema(), newAppResource[kubernetesNodesResourceModel](client.KubernetesNodesResource(), "kubernetes_nodes", kubernetesNodesSchema)().(resource.ResourceWithValidateConfig)},
			} {
				state := tfsdk.State{Schema: fixture.schema, Raw: tftypes.NewValue(fixture.schema.Type().TerraformType(context.Background()), nil)}
				if diagnostics := state.Set(context.Background(), fixture.model); diagnostics.HasError() {
					t.Fatal(diagnostics)
				}
				response := &resource.ValidateConfigResponse{}
				fixture.res.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: fixture.schema, Raw: state.Raw}}, response)
				want := !gpu.Elements()[0].(types.Object).Attributes()["class_name"].IsUnknown()
				if response.Diagnostics.HasError() != want {
					t.Fatalf("%s config diagnostics=%v, want error=%v", name, response.Diagnostics, want)
				}
			}
		})
	}
}

func TestTenantGPUUnsetNestedFlagAndImport(t *testing.T) {
	t.Parallel()
	for _, gpu := range []types.Object{types.ObjectNull(tenantGPUObjectType()), types.ObjectUnknown(tenantGPUObjectType()), types.ObjectValueMust(tenantGPUObjectType(), map[string]attr.Value{"vgpu": types.BoolNull()}), types.ObjectValueMust(tenantGPUObjectType(), map[string]attr.Value{"vgpu": types.BoolUnknown()})} {
		t.Run(gpu.String(), func(t *testing.T) {
			t.Parallel()
			model := fullModel()
			model.GPU = gpu
			app, diagnostics := model.expand(context.Background())
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if wire, ok := app.Spec["gpu"].(map[string]any); ok {
				if _, present := wire["vgpu"]; present {
					t.Fatal("unset/unknown flag must be omitted")
				}
			}
		})
	}
	app := &client.Application{Spec: map[string]any{"gpu": map[string]any{"vgpu": true}}}
	imported := tenantResourceModel{}
	if diagnostics := imported.flatten(app); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if !imported.GPU.Attributes()["vgpu"].Equal(types.BoolValue(true)) {
		t.Fatal("import lost configured entitlement")
	}
}
