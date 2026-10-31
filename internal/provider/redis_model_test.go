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
		MaxmemoryPolicy: types.StringValue("allkeys-lru"),
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

	// maxmemoryPolicy is newer than the pinned API; its wire key is checked separately.
	if got.Spec["maxmemoryPolicy"] != "allkeys-lru" {
		t.Errorf("spec.maxmemoryPolicy = %v, want allkeys-lru", got.Spec["maxmemoryPolicy"])
	}
	delete(emitted, "maxmemoryPolicy")

	// Added by the pinned API and not modelled yet.
	assertSpecCoverage(t, emitted, redis.ConfigSpec{}, "tls")
}

func TestRedisMaxmemoryPolicyRoundTrip(t *testing.T) {
	t.Parallel()

	for _, policy := range []string{
		"noeviction", "allkeys-lru", "allkeys-lfu", "volatile-lru", "volatile-lfu",
		"allkeys-random", "volatile-random", "volatile-ttl",
	} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()

			model := fullRedisModel()
			model.MaxmemoryPolicy = types.StringValue(policy)
			app, diags := model.expand(context.Background())
			if diags.HasError() {
				t.Fatalf("expand diagnostics: %v", diags)
			}
			if got := app.Spec["maxmemoryPolicy"]; got != policy {
				t.Errorf("spec.maxmemoryPolicy = %v, want %s", got, policy)
			}
			var out redisModel
			if diags := out.flatten(app); diags.HasError() {
				t.Fatalf("flatten diagnostics: %v", diags)
			}
			if !out.MaxmemoryPolicy.Equal(model.MaxmemoryPolicy) {
				t.Errorf("maxmemory_policy = %v, want %v", out.MaxmemoryPolicy, model.MaxmemoryPolicy)
			}
		})
	}
}

func TestRedisMaxmemoryPolicyExpandUnset(t *testing.T) {
	t.Parallel()

	for _, value := range []types.String{types.StringNull(), types.StringUnknown()} {
		t.Run(value.String(), func(t *testing.T) {
			t.Parallel()

			model := fullRedisModel()
			model.MaxmemoryPolicy = value
			app, diags := model.expand(context.Background())
			if diags.HasError() {
				t.Fatalf("expand diagnostics: %v", diags)
			}
			if _, present := app.Spec["maxmemoryPolicy"]; present {
				t.Errorf("unset policy emitted %v, want key omitted", app.Spec["maxmemoryPolicy"])
			}
		})
	}
}

func TestRedisMaxmemoryPolicyFlattenLegacy(t *testing.T) {
	t.Parallel()

	for _, spec := range []map[string]any{nil, {}, {"maxmemoryPolicy": nil}} {
		model := fullRedisModel()
		app := &client.Application{Name: "cache", Namespace: "tenant-root", Spec: spec}
		if diags := model.flatten(app); diags.HasError() {
			t.Fatalf("flatten diagnostics: %v", diags)
		}
		if got := model.MaxmemoryPolicy.ValueString(); got != "noeviction" {
			t.Errorf("maxmemory_policy = %q, want noeviction, not prior configured allkeys-lru", got)
		}
	}
}
