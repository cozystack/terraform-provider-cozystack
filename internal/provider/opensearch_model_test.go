package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/opensearch"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullOpensearchModel() opensearchModel {
	usersType := types.ObjectType{AttrTypes: opensearchUserObjectType()}

	return opensearchModel{
		Name:                 types.StringValue("search"),
		Namespace:            types.StringValue("tenant-root"),
		Replicas:             types.Int64Value(3),
		Resources:            types.ObjectNull(resourcesObjectType()),
		ResourcesPreset:      types.StringValue("c1.medium"),
		Size:                 types.StringValue("10Gi"),
		StorageClass:         types.StringValue(""),
		External:             types.BoolValue(false),
		TopologySpreadPolicy: types.StringValue("soft"),
		Version:              types.StringValue("v2"),
		Users: types.MapValueMust(usersType, map[string]attr.Value{
			"app": types.ObjectValueMust(opensearchUserObjectType(), map[string]attr.Value{
				"password": types.StringValue("pw"),
				"roles":    types.ListValueMust(types.StringType, []attr.Value{types.StringValue("admin")}),
			}),
		}),
	}
}

func TestOpensearchExpand_Users(t *testing.T) {
	t.Parallel()

	model := fullOpensearchModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	users, _ := got.Spec["users"].(map[string]any)
	app, _ := users["app"].(map[string]any)
	roles, ok := app["roles"].([]any)
	if !ok || len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("users.app.roles = %v, want [admin]", app["roles"])
	}
}

func TestOpensearchFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "search",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":             int64(3),
			"resourcesPreset":      "c1.medium",
			"size":                 "10Gi",
			"storageClass":         "",
			"external":             false,
			"topologySpreadPolicy": "hard",
			"version":              "v3",
			"resources":            map[string]any{},
			"users":                map[string]any{"app": map[string]any{"password": "x", "roles": []any{"reader"}}},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model opensearchModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.TopologySpreadPolicy.ValueString() != "hard" {
		t.Errorf("topology_spread_policy = %q, want hard", model.TopologySpreadPolicy.ValueString())
	}
	if model.Version.ValueString() != "v3" {
		t.Errorf("version = %q, want v3", model.Version.ValueString())
	}
}

func TestOpensearchExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullOpensearchModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, opensearch.ConfigSpec{}, "images", "nodeRoles", "dashboards")
}
