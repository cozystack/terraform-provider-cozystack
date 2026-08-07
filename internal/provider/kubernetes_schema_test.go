package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// kubernetesRaw renders a model into the schema's raw value, the shape the
// framework hands to validators and plan modifiers.
func kubernetesRaw(ctx context.Context, t *testing.T, model kubernetesModel) tftypes.Value {
	t.Helper()

	schema := kubernetesSchema()

	state := tfsdk.State{
		Schema: schema,
		Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), nil),
	}

	resourceModel := kubernetesResourceModel{
		kubernetesModel: model,
		WaitForReady:    types.BoolValue(false),
		WaitTimeout:     types.StringValue("10m"),
	}

	if diags := state.Set(ctx, &resourceModel); diags.HasError() {
		t.Fatalf("state.Set: %v", diags)
	}

	return state.Raw
}

// kubernetesConfig renders a model into a tfsdk.Config against the resource
// schema, so attribute validators can be exercised against a real config value.
func kubernetesConfig(ctx context.Context, t *testing.T, model kubernetesModel) tfsdk.Config {
	t.Helper()

	return tfsdk.Config{Schema: kubernetesSchema(), Raw: kubernetesRaw(ctx, t, model)}
}

// nodeGroupWithResources rebuilds the fixture's md0 group with an explicit
// resources object.
func nodeGroupWithResources(t *testing.T, model kubernetesModel, resources types.Object) kubernetesModel {
	t.Helper()

	group, _ := model.NodeGroups.Elements()["md0"].(types.Object)

	attributes := map[string]attr.Value{}
	for name, value := range group.Attributes() {
		attributes[name] = value
	}

	attributes["resources"] = resources

	model.NodeGroups = types.MapValueMust(
		types.ObjectType{AttrTypes: k8sNodeGroupObjectType()},
		map[string]attr.Value{"md0": types.ObjectValueMust(k8sNodeGroupObjectType(), attributes)},
	)

	return model
}

// validateNodeGroupCPU runs the validators declared on
// node_groups[*].resources.cpu against a config, the way the framework does.
func validateNodeGroupCPU(ctx context.Context, t *testing.T, config tfsdk.Config, value types.String) bool {
	t.Helper()

	cpuPath := path.Root("node_groups").AtMapKey("md0").AtName("resources").AtName(attrCPU)

	attribute, diags := config.Schema.AttributeAtPath(ctx, cpuPath)
	if diags.HasError() {
		t.Fatalf("AttributeAtPath: %v", diags)
	}

	stringAttribute, ok := attribute.(rschema.StringAttribute)
	if !ok {
		t.Fatalf("cpu attribute is %T, want a string attribute", attribute)
	}

	request := validator.StringRequest{
		Path:           cpuPath,
		PathExpression: cpuPath.Expression(),
		Config:         config,
		ConfigValue:    value,
	}

	failed := false

	for _, check := range stringAttribute.Validators {
		response := &validator.StringResponse{}
		check.ValidateString(ctx, request, response)

		if response.Diagnostics.HasError() {
			failed = true
		}
	}

	return failed
}

// Upstream sizes a node group by instance_type unless both cpu and memory are
// given, and fails the chart render on a half-filled block. The schema binds the
// two so that surfaces at plan time instead.
func TestKubernetesNodeGroupResourcesRequireBothOrNeither(t *testing.T) {
	t.Parallel()

	cpuOnly := types.ObjectValueMust(resourcesObjectType(), map[string]attr.Value{
		attrCPU:    types.StringValue("4"),
		attrMemory: types.StringNull(),
	})

	ctx := context.Background()

	config := kubernetesConfig(ctx, t, nodeGroupWithResources(t, fullKubernetesModel(), cpuOnly))
	if !validateNodeGroupCPU(ctx, t, config, types.StringValue("4")) {
		t.Errorf("cpu without memory validated cleanly, want an error")
	}

	both := types.ObjectValueMust(resourcesObjectType(), map[string]attr.Value{
		attrCPU:    types.StringValue("4"),
		attrMemory: types.StringValue("8Gi"),
	})

	config = kubernetesConfig(ctx, t, nodeGroupWithResources(t, fullKubernetesModel(), both))
	if validateNodeGroupCPU(ctx, t, config, types.StringValue("4")) {
		t.Errorf("cpu with memory produced an error, want it accepted")
	}
}

// The model binds to the schema by tfsdk tag; a tag that names no attribute (or
// an attribute with no tag) is a runtime failure the expand/flatten tests never
// reach. Round-tripping the full fixture through the schema catches it.
func TestKubernetesResourceModelRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := kubernetesSchema()

	state := tfsdk.State{
		Schema: schema,
		Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), nil),
	}

	in := kubernetesResourceModel{
		kubernetesModel: fullKubernetesModel(),
		WaitForReady:    types.BoolValue(true),
		WaitTimeout:     types.StringValue("15m"),
	}

	in.ID = types.StringValue("tenant-root/cluster")
	in.Ready = types.BoolValue(true)
	in.ChartVersion = types.StringValue("1.6.1")
	in.UID = types.StringValue("6f0b1f9c-0000-4000-8000-000000000000")
	in.Kubeconfig = types.StringNull()

	if diags := state.Set(ctx, &in); diags.HasError() {
		t.Fatalf("state.Set: %v", diags)
	}

	var out kubernetesResourceModel

	if diags := state.Get(ctx, &out); diags.HasError() {
		t.Fatalf("state.Get: %v", diags)
	}

	if out.Talos.IsNull() || out.OIDC.IsNull() || out.NodeHealthCheck.IsNull() {
		t.Errorf("nested blocks lost in the round trip: talos=%v oidc=%v node_health_check=%v",
			out.Talos, out.OIDC, out.NodeHealthCheck)
	}

	group, _ := out.NodeGroups.Elements()["md0"].(types.Object)

	timeout, _ := group.Attributes()["node_startup_timeout"].(types.String)
	if timeout.ValueString() != "20m" {
		t.Errorf("node group startup timeout = %q, want 20m", timeout.ValueString())
	}
}

// The 1.6 servers reject v1.30; advertising it would let a config plan cleanly
// and fail at apply.
func TestKubernetesVersionValidatorRejectsRetiredReleases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	attribute, ok := kubernetesSchema().Attributes[attrVersion].(rschema.StringAttribute)
	if !ok {
		t.Fatalf("version attribute is not a string attribute")
	}

	for value, wantError := range map[string]bool{"v1.35": false, "v1.31": false, "v1.30": true} {
		failed := false

		for _, check := range attribute.Validators {
			response := &validator.StringResponse{}
			check.ValidateString(ctx, validator.StringRequest{
				Path:        path.Root(attrVersion),
				ConfigValue: types.StringValue(value),
			}, response)

			if response.Diagnostics.HasError() {
				failed = true
			}
		}

		if failed != wantError {
			t.Errorf("version %q rejected = %v, want %v", value, failed, wantError)
		}
	}
}

// storage_class is marked immutable upstream, but the aggregated apiserver does
// not evaluate that rule: it accepts the write, the release records the new
// class, and the PVCs keep the old one forever, because Kubernetes fixes a
// PVC's storageClassName at creation and editing volumeClaimTemplates never
// migrates existing data. A silent, permanent divergence between state and
// reality is worse than a rejection, so the provider makes the intent explicit
// and plans a replacement.
func TestKubernetesStorageClassRequiresReplace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := kubernetesSchema()

	attribute, ok := schema.Attributes[attrStorageClass].(rschema.StringAttribute)
	if !ok {
		t.Fatalf("storage_class is not a string attribute")
	}

	before := fullKubernetesModel()

	after := fullKubernetesModel()
	after.StorageClass = types.StringValue("local")

	tests := []struct {
		name  string
		plan  kubernetesModel
		value types.String
		want  bool
	}{
		{name: "unchanged class plans in place", plan: before, value: before.StorageClass},
		{name: "changed class requires replacement", plan: after, value: after.StorageClass, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := planmodifier.StringRequest{
				Path:           path.Root(attrStorageClass),
				PathExpression: path.Root(attrStorageClass).Expression(),
				State:          tfsdk.State{Schema: schema, Raw: kubernetesRaw(ctx, t, before)},
				StateValue:     before.StorageClass,
				Plan:           tfsdk.Plan{Schema: schema, Raw: kubernetesRaw(ctx, t, tt.plan)},
				PlanValue:      tt.value,
				Config:         kubernetesConfig(ctx, t, tt.plan),
				ConfigValue:    tt.value,
			}

			response := &planmodifier.StringResponse{PlanValue: request.PlanValue}

			for _, modifier := range attribute.PlanModifiers {
				modifier.PlanModifyString(ctx, request, response)
			}

			if response.RequiresReplace != tt.want {
				t.Errorf("RequiresReplace = %v, want %v", response.RequiresReplace, tt.want)
			}
		})
	}
}
