package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/nats"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullNatsModel() natsModel {
	usersType := types.ObjectType{AttrTypes: passwordUserObjectType()}

	return natsModel{
		Name:            types.StringValue("bus"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(2),
		Resources:       types.ObjectNull(resourcesObjectType()),
		ResourcesPreset: types.StringValue("t1.nano"),
		StorageClass:    types.StringValue(""),
		External:        types.BoolValue(false),
		TLS:             tlsBlock(true),
		Users: types.MapValueMust(usersType, map[string]attr.Value{
			"app": types.ObjectValueMust(passwordUserObjectType(), map[string]attr.Value{"password": types.StringValue("pw")}),
		}),
	}
}

func TestNatsExpand_Users(t *testing.T) {
	t.Parallel()

	model := fullNatsModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	users, _ := got.Spec["users"].(map[string]any)
	if _, ok := users["app"]; !ok {
		t.Errorf("users missing app")
	}
}

func TestNatsFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "bus",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(2),
			"resourcesPreset": "t1.nano",
			"storageClass":    "",
			"external":        true,
			"resources":       map[string]any{},
			"users":           map[string]any{"app": map[string]any{"password": "x"}},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model natsModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !model.External.ValueBool() {
		t.Errorf("external = false, want true")
	}
}

func TestNatsExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullNatsModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, nats.ConfigSpec{}, "jetstream", "config")
}
