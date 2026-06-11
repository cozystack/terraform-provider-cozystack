package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/mongodb"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullMongodbModel() mongodbModel {
	usersType := types.ObjectType{AttrTypes: passwordUserObjectType()}

	return mongodbModel{
		Name:            types.StringValue("docs"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(3),
		Resources:       types.ObjectNull(resourcesObjectType()),
		ResourcesPreset: types.StringValue("t1.small"),
		Size:            types.StringValue("10Gi"),
		StorageClass:    types.StringValue(""),
		External:        types.BoolValue(false),
		Version:         types.StringValue("v8"),
		Sharding:        types.BoolValue(true),
		Users: types.MapValueMust(usersType, map[string]attr.Value{
			"app": types.ObjectValueMust(passwordUserObjectType(), map[string]attr.Value{"password": types.StringValue("pw")}),
		}),
		Databases: types.MapNull(types.ObjectType{AttrTypes: rolesEntryObjectType()}),
	}
}

func TestMongodbExpand_Sharding(t *testing.T) {
	t.Parallel()

	model := fullMongodbModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec["sharding"] != true {
		t.Errorf("spec.sharding = %v, want true", got.Spec["sharding"])
	}

	users, _ := got.Spec["users"].(map[string]any)
	if _, ok := users["app"]; !ok {
		t.Errorf("users missing app")
	}
}

func TestMongodbFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "docs",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(3),
			"resourcesPreset": "t1.small",
			"size":            "10Gi",
			"storageClass":    "",
			"external":        false,
			"version":         "v7",
			"sharding":        false,
			"resources":       map[string]any{},
			"users":           map[string]any{"app": map[string]any{"password": "x"}},
			"databases":       map[string]any{},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model mongodbModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Version.ValueString() != "v7" {
		t.Errorf("version = %q, want v7", model.Version.ValueString())
	}
	if model.Sharding.ValueBool() {
		t.Errorf("sharding = true, want false")
	}
}

func TestMongodbExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullMongodbModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, mongodb.ConfigSpec{}, "backup", "bootstrap", "shardingConfig")
}
