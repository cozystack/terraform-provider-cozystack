package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/clickhouse"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullClickhouseModel() clickhouseModel {
	usersType := types.ObjectType{AttrTypes: clickhouseUserObjectType()}

	return clickhouseModel{
		Name:            types.StringValue("ch"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(2),
		Shards:          types.Int64Value(1),
		Resources:       types.ObjectNull(resourcesObjectType()),
		ResourcesPreset: types.StringValue("t1.small"),
		Size:            types.StringValue("10Gi"),
		StorageClass:    types.StringValue(""),
		LogStorageSize:  types.StringValue("2Gi"),
		LogTTL:          types.Int64Value(15),
		Users: types.MapValueMust(usersType, map[string]attr.Value{
			"reader": types.ObjectValueMust(clickhouseUserObjectType(), map[string]attr.Value{
				"password": types.StringValue("pw"),
				"readonly": types.BoolValue(true),
			}),
		}),
	}
}

func TestClickhouseExpand_Users(t *testing.T) {
	t.Parallel()

	model := fullClickhouseModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec["shards"] != int64(1) {
		t.Errorf("spec.shards = %v, want 1", got.Spec["shards"])
	}
	if got.Spec["logTTL"] != int64(15) {
		t.Errorf("spec.logTTL = %v, want 15", got.Spec["logTTL"])
	}

	users, _ := got.Spec["users"].(map[string]any)
	reader, _ := users["reader"].(map[string]any)
	if reader["readonly"] != true || reader["password"] != "pw" {
		t.Errorf("users.reader = %v, want {password:pw, readonly:true}", reader)
	}
}

func TestClickhouseFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "ch",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(2),
			"shards":          int64(2),
			"resourcesPreset": "t1.small",
			"size":            "10Gi",
			"storageClass":    "",
			"logStorageSize":  "2Gi",
			"logTTL":          int64(30),
			"resources":       map[string]any{},
			"users":           map[string]any{"u": map[string]any{"password": "x", "readonly": false}},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model clickhouseModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Shards.ValueInt64() != 2 {
		t.Errorf("shards = %d, want 2", model.Shards.ValueInt64())
	}
	if model.LogTTL.ValueInt64() != 30 {
		t.Errorf("log_ttl = %d, want 30", model.LogTTL.ValueInt64())
	}
}

func TestClickhouseExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullClickhouseModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, clickhouse.ConfigSpec{}, "backup", "clickhouseKeeper")
}
