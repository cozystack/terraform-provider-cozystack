package provider

import (
	"context"
	"testing"

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

	if out.Talos.IsNull() || out.OIDC.IsNull() {
		t.Errorf("nested blocks lost in the round trip: talos=%v oidc=%v", out.Talos, out.OIDC)
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
		name        string
		plan        kubernetesModel
		value       types.String
		configValue types.String
		want        bool
	}{
		{
			name:        "unchanged class plans in place",
			plan:        before,
			value:       before.StorageClass,
			configValue: before.StorageClass,
		},
		{
			name:        "changed class requires replacement",
			plan:        after,
			value:       after.StorageClass,
			configValue: after.StorageClass,
			want:        true,
		},
		{
			// An imported cluster on a non-default class with a configuration that
			// never mentions storage_class: nothing may plan a change here at all,
			// and an unconditional modifier would turn one into a destroy nobody
			// asked for. The attribute carries no default, so the plan keeps the
			// prior value and this path stays theoretical.
			name:        "unconfigured change is not a replacement",
			plan:        after,
			value:       after.StorageClass,
			configValue: types.StringNull(),
		},
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
				ConfigValue:    tt.configValue,
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

// The data source binds the same model to a different schema, and it is the
// surface the documented `data.cozystack_kubernetes.….talos.version` output
// reads. A tfsdk tag that names no attribute there fails at runtime, and no
// expand/flatten test would reach it.
func TestKubernetesDataSourceModelRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := kubernetesDataSourceSchema()

	state := tfsdk.State{
		Schema: schema,
		Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), nil),
	}

	in := fullKubernetesModel()
	in.ID = types.StringValue("tenant-root/cluster")
	in.Ready = types.BoolValue(true)
	in.ChartVersion = types.StringValue("1.6.1")
	in.UID = types.StringValue("6f0b1f9c-0000-4000-8000-000000000000")
	in.Kubeconfig = types.StringNull()

	if diags := state.Set(ctx, &in); diags.HasError() {
		t.Fatalf("state.Set: %v", diags)
	}

	var out kubernetesModel

	if diags := state.Get(ctx, &out); diags.HasError() {
		t.Fatalf("state.Get: %v", diags)
	}

	// The two attributes the data-source example publishes as outputs.
	talos, _ := out.Talos.Attributes()[attrVersion].(types.String)
	if talos.ValueString() != "v1.13.6" {
		t.Errorf("talos.version = %q, want v1.13.6", talos.ValueString())
	}

	oidc, _ := out.OIDC.Attributes()["mode"].(types.String)
	if oidc.ValueString() != "System" {
		t.Errorf("oidc.mode = %q, want System", oidc.ValueString())
	}

	if out.ControlPlane.IsNull() || out.Images.IsNull() {
		t.Errorf("blocks lost through the data-source schema: control_plane=%v images=%v",
			out.ControlPlane, out.Images)
	}
}
