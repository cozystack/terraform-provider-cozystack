package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/postgresql"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullPostgresqlModel() postgresqlModel {
	usersType := types.ObjectType{AttrTypes: pgUserObjectType()}

	return postgresqlModel{
		Name:            types.StringValue("pg"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(2),
		Resources:       types.ObjectNull(resourcesObjectType()),
		ResourcesPreset: types.StringValue("t1.micro"),
		Size:            types.StringValue("10Gi"),
		StorageClass:    types.StringValue(""),
		External:        types.BoolValue(false),
		TLS:             tlsBlock(true),
		Version:         types.StringValue("v18"),
		Users: types.MapValueMust(usersType, map[string]attr.Value{
			"app": types.ObjectValueMust(pgUserObjectType(), map[string]attr.Value{
				"password":    types.StringValue("pw"),
				"replication": types.BoolValue(true),
			}),
		}),
		Databases: types.MapNull(types.ObjectType{AttrTypes: pgDatabaseObjectType()}),
	}
}

func TestPostgresqlExpand_Users(t *testing.T) {
	t.Parallel()

	model := fullPostgresqlModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	users, _ := got.Spec["users"].(map[string]any)
	app, _ := users["app"].(map[string]any)
	if app["replication"] != true || app["password"] != "pw" {
		t.Errorf("users.app = %v, want {password:pw, replication:true}", app)
	}
}

func TestPostgresqlFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "pg",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(2),
			"resourcesPreset": "t1.micro",
			"size":            "10Gi",
			"storageClass":    "",
			"external":        false,
			"version":         "v16",
			"resources":       map[string]any{},
			"users":           map[string]any{"app": map[string]any{"password": "x", "replication": false}},
			"databases": map[string]any{
				"appdb": map[string]any{"extensions": []any{"postgis"}, "roles": map[string]any{"admin": []any{"app"}}},
			},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model postgresqlModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Version.ValueString() != "v16" {
		t.Errorf("version = %q, want v16", model.Version.ValueString())
	}

	databases := model.Databases.Elements()
	if _, ok := databases["appdb"]; !ok {
		t.Errorf("databases missing appdb")
	}
}

func TestPostgresqlExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullPostgresqlModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, postgresql.ConfigSpec{}, "postgresql", "quorum", "backup", "bootstrap")
}
