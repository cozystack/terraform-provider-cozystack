package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/clickhouse"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
		Version:         types.StringValue("v24.9"),
		Backup:          backupBlock(true),
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
	if got.Spec["version"] != "v24.9" {
		t.Errorf("spec.version = %v, want v24.9", got.Spec["version"])
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
			"version":         "v25.8",
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
	if model.Version.ValueString() != "v25.8" {
		t.Errorf("version = %q, want v25.8", model.Version.ValueString())
	}
}

// A server that predates the field returns no version. It must flatten to null,
// not "", or the next update would write an empty version into the spec.
func TestClickhouseFlattenWithoutVersionIsNull(t *testing.T) {
	t.Parallel()

	var model clickhouseModel

	app := &client.Application{Name: "ch", Namespace: "tenant-root", Spec: map[string]any{}}
	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !model.Version.IsNull() {
		t.Errorf("version = %v, want null", model.Version)
	}
}

// plannedClickhouseVersion replays the framework's plan for `version`: a
// configured value is kept, an unconfigured one takes the schema default when
// there is one and is otherwise unknown, and the plan modifiers run last.
func plannedClickhouseVersion(ctx context.Context, t *testing.T, stateValue, configValue types.String) types.String {
	t.Helper()

	attribute, ok := clickhouseSchema().Attributes[attrVersion].(rschema.StringAttribute)
	if !ok {
		t.Fatalf("version is not a string attribute")
	}

	planValue := configValue

	if configValue.IsNull() {
		planValue = types.StringUnknown()

		if attribute.Default != nil {
			response := &defaults.StringResponse{}
			attribute.Default.DefaultString(ctx, defaults.StringRequest{}, response)
			planValue = response.PlanValue
		}
	}

	request := planmodifier.StringRequest{StateValue: stateValue, PlanValue: planValue, ConfigValue: configValue}
	response := &planmodifier.StringResponse{PlanValue: planValue}

	for _, modifier := range attribute.PlanModifiers {
		modifier.PlanModifyString(ctx, request, response)
	}

	return response.PlanValue
}

// Upstream calls a ClickHouse downgrade unsafe, and the server default v24.9 is
// the oldest release in the enum. An update replaces the whole spec, so an
// unconfigured version must follow the running instance, never a
// provider-side default.
func TestClickhouseVersionPlannedIntoSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		stateValue  types.String
		configValue types.String
		want        types.String
	}{
		{
			name:        "unset version without a prior value is left to the server",
			stateValue:  types.StringNull(),
			configValue: types.StringNull(),
			want:        types.StringNull(),
		},
		{
			name:        "deleted version line after an upgrade keeps the running version",
			stateValue:  types.StringValue("v25.8"),
			configValue: types.StringNull(),
			want:        types.StringValue("v25.8"),
		},
		{
			name:        "explicit older version is emitted as given",
			stateValue:  types.StringValue("v25.8"),
			configValue: types.StringValue("v24.9"),
			want:        types.StringValue("v24.9"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			model := fullClickhouseModel()
			model.Version = plannedClickhouseVersion(ctx, t, tt.stateValue, tt.configValue)

			got, diags := model.expand(ctx)
			if diags.HasError() {
				t.Fatalf("expand diagnostics: %v", diags)
			}

			value, present := got.Spec[attrVersion]

			switch {
			case tt.want.IsNull() && present:
				t.Errorf("spec.version = %v, want the key left out", value)
			case !tt.want.IsNull() && value != tt.want.ValueString():
				t.Errorf("spec.version = %v, want %s", value, tt.want.ValueString())
			}
		})
	}
}

// Every upstream release is accepted, and a value outside the enum fails at
// plan time rather than at apply.
func TestClickhouseVersionValidator(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	attribute, ok := clickhouseSchema().Attributes[attrVersion].(rschema.StringAttribute)
	if !ok {
		t.Fatalf("version attribute is not a string attribute")
	}

	for value, wantError := range map[string]bool{
		"v25.8": false, "v25.3": false, "v24.9": false, "25.8": true, "v24.8": true,
	} {
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

	assertSpecCoverage(t, emitted, clickhouse.ConfigSpec{}, "clickhouseKeeper")
}
