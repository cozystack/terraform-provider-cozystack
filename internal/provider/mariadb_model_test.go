package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/mariadb"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullMariadbModel() mariadbModel {
	usersType := types.ObjectType{AttrTypes: mariadbUserObjectType()}

	return mariadbModel{
		Name:            types.StringValue("db"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(2),
		Resources:       types.ObjectNull(resourcesObjectType()),
		ResourcesPreset: types.StringValue("t1.nano"),
		Size:            types.StringValue("10Gi"),
		StorageClass:    types.StringValue(""),
		External:        types.BoolValue(false),
		Version:         types.StringValue("v11.8"),
		Users: types.MapValueMust(usersType, map[string]attr.Value{
			"app": types.ObjectValueMust(mariadbUserObjectType(), map[string]attr.Value{
				"max_user_connections": types.Int64Value(10),
				"password":             types.StringValue("pw"),
			}),
		}),
		Databases: types.MapNull(types.ObjectType{AttrTypes: rolesEntryObjectType()}),
	}
}

func TestMariadbExpand_Users(t *testing.T) {
	t.Parallel()

	model := fullMariadbModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	users, _ := got.Spec["users"].(map[string]any)
	app, _ := users["app"].(map[string]any)
	if app["maxUserConnections"] != int64(10) {
		t.Errorf("users.app.maxUserConnections = %v, want 10", app["maxUserConnections"])
	}
	if app["password"] != "pw" {
		t.Errorf("users.app.password = %v, want pw", app["password"])
	}
}

func TestMariadbFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "db",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(2),
			"resourcesPreset": "t1.nano",
			"size":            "10Gi",
			"storageClass":    "",
			"external":        false,
			"version":         "v10.11",
			"resources":       map[string]any{},
			"users":           map[string]any{"app": map[string]any{"maxUserConnections": int64(5), "password": "x"}},
			"databases":       map[string]any{"appdb": map[string]any{"roles": map[string]any{"admin": []any{"app"}}}},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model mariadbModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Version.ValueString() != "v10.11" {
		t.Errorf("version = %q, want v10.11", model.Version.ValueString())
	}
	if _, ok := model.Databases.Elements()["appdb"]; !ok {
		t.Errorf("databases missing appdb")
	}
}

func TestMariadbExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullMariadbModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	// The deprecated inline backup block is intentionally not managed.
	assertSpecCoverage(t, emitted, mariadb.ConfigSpec{}, "backup")
}
