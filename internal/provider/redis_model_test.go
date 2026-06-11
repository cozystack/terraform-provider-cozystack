package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/redis"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullRedisModel() redisModel {
	return redisModel{
		Name:            types.StringValue("cache"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(3),
		Size:            types.StringValue("2Gi"),
		StorageClass:    types.StringValue("replicated"),
		External:        types.BoolValue(false),
		Version:         types.StringValue("v8"),
		AuthEnabled:     types.BoolValue(true),
		ResourcesPreset: types.StringValue("t1.small"),
		Resources: types.ObjectValueMust(resourcesObjectType(), map[string]attr.Value{
			"cpu":    types.StringValue("500m"),
			"memory": types.StringValue("512Mi"),
		}),
	}
}

func TestRedisExpand_CoreFields(t *testing.T) {
	t.Parallel()

	model := fullRedisModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Name != "cache" || got.Namespace != "tenant-root" {
		t.Errorf("identity = %s/%s, want tenant-root/cache", got.Namespace, got.Name)
	}
	if got.Spec["replicas"] != int64(3) {
		t.Errorf("spec.replicas = %v (%T), want int64 3", got.Spec["replicas"], got.Spec["replicas"])
	}
	if got.Spec["size"] != "2Gi" {
		t.Errorf("spec.size = %v, want 2Gi", got.Spec["size"])
	}
	if got.Spec["version"] != "v8" {
		t.Errorf("spec.version = %v, want v8", got.Spec["version"])
	}
	if got.Spec["authEnabled"] != true {
		t.Errorf("spec.authEnabled = %v, want true", got.Spec["authEnabled"])
	}
}

func TestRedisExpand_Resources(t *testing.T) {
	t.Parallel()

	model := fullRedisModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	resources, ok := got.Spec["resources"].(map[string]any)
	if !ok {
		t.Fatalf("spec.resources type = %T, want map[string]any", got.Spec["resources"])
	}
	if resources["cpu"] != "500m" || resources["memory"] != "512Mi" {
		t.Errorf("resources = %v, want cpu=500m memory=512Mi", resources)
	}
}

func TestRedisExpand_NullResourcesYieldsEmptyMap(t *testing.T) {
	t.Parallel()

	model := fullRedisModel()
	model.Resources = types.ObjectNull(resourcesObjectType())

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	resources, ok := got.Spec["resources"].(map[string]any)
	if !ok || len(resources) != 0 {
		t.Errorf("resources = %v, want empty map", got.Spec["resources"])
	}
}

func TestRedisFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "cache",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(2),
			"size":            "1Gi",
			"storageClass":    "",
			"external":        false,
			"version":         "v7",
			"authEnabled":     true,
			"resourcesPreset": "t1.nano",
			"resources":       map[string]any{"cpu": "250m"},
		},
		Status: client.ApplicationStatus{Version: "0.6.1", Ready: true},
	}

	var model redisModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "tenant-root/cache" {
		t.Errorf("id = %q, want tenant-root/cache", model.ID.ValueString())
	}
	if model.Replicas.ValueInt64() != 2 {
		t.Errorf("replicas = %d, want 2", model.Replicas.ValueInt64())
	}
	if model.Version.ValueString() != "v7" {
		t.Errorf("version = %q, want v7", model.Version.ValueString())
	}
	if !model.Ready.ValueBool() {
		t.Errorf("ready = false, want true")
	}
	if model.ChartVersion.ValueString() != "0.6.1" {
		t.Errorf("chart_version = %q, want 0.6.1", model.ChartVersion.ValueString())
	}

	resources := model.Resources.Attributes()
	if cpu := resources["cpu"]; cpu == nil || cpu.(types.String).ValueString() != "250m" {
		t.Errorf("resources.cpu = %v, want 250m", cpu)
	}
	if memory := resources["memory"]; memory == nil || !memory.(types.String).IsNull() {
		t.Errorf("resources.memory = %v, want null", memory)
	}
}

func TestRedisFlatten_EmptyResourcesIsNull(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "cache",
		Namespace: "tenant-root",
		Spec:      map[string]any{"resources": map[string]any{}},
	}

	var model redisModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !model.Resources.IsNull() {
		t.Errorf("resources = %v, want null for an empty block", model.Resources)
	}
}

// TestRedisExpandKeysMatchConfigSpec guards against field-name drift between the
// provider and the pinned redis.ConfigSpec.
func TestRedisExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullRedisModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	tags := configSpecJSONTags(redis.ConfigSpec{})

	for key := range emitted {
		if !tags[key] {
			t.Errorf("expand emits %q, which is not a redis.ConfigSpec json tag", key)
		}
	}

	for tag := range tags {
		if !emitted[tag] {
			t.Errorf("redis.ConfigSpec has %q but expand does not emit it", tag)
		}
	}
}
