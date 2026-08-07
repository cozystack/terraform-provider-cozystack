package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/qdrant"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullQdrantModel() qdrantModel {
	return qdrantModel{
		Name:            types.StringValue("vectors"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(2),
		Size:            types.StringValue("20Gi"),
		StorageClass:    types.StringValue("replicated"),
		External:        types.BoolValue(false),
		TLS:             tlsBlock(true),
		ResourcesPreset: types.StringValue("t1.medium"),
		Resources: types.ObjectValueMust(resourcesObjectType(), map[string]attr.Value{
			"cpu":    types.StringValue("1"),
			"memory": types.StringValue("2Gi"),
		}),
	}
}

func TestQdrantExpand_CoreFields(t *testing.T) {
	t.Parallel()

	model := fullQdrantModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Name != "vectors" || got.Namespace != "tenant-root" {
		t.Errorf("identity = %s/%s, want tenant-root/vectors", got.Namespace, got.Name)
	}
	if got.Spec["replicas"] != int64(2) {
		t.Errorf("spec.replicas = %v, want 2", got.Spec["replicas"])
	}
	if got.Spec["resourcesPreset"] != "t1.medium" {
		t.Errorf("spec.resourcesPreset = %v, want t1.medium", got.Spec["resourcesPreset"])
	}

	resources, ok := got.Spec["resources"].(map[string]any)
	if !ok || resources["cpu"] != "1" || resources["memory"] != "2Gi" {
		t.Errorf("spec.resources = %v, want cpu=1 memory=2Gi", got.Spec["resources"])
	}
}

func TestQdrantFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "vectors",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(1),
			"size":            "10Gi",
			"storageClass":    "",
			"external":        true,
			"resourcesPreset": "t1.small",
			"resources":       map[string]any{},
		},
		Status: client.ApplicationStatus{Version: "0.3.0", Ready: true},
	}

	var model qdrantModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "tenant-root/vectors" {
		t.Errorf("id = %q, want tenant-root/vectors", model.ID.ValueString())
	}
	if !model.External.ValueBool() {
		t.Errorf("external = false, want true")
	}
	if !model.Resources.IsNull() {
		t.Errorf("empty resources should flatten to null")
	}
	if model.ChartVersion.ValueString() != "0.3.0" {
		t.Errorf("chart_version = %q, want 0.3.0", model.ChartVersion.ValueString())
	}
}

// TestQdrantExpandKeysMatchConfigSpec guards against field-name drift.
func TestQdrantExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullQdrantModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	tags := configSpecJSONTags(qdrant.ConfigSpec{})

	for key := range emitted {
		if !tags[key] {
			t.Errorf("expand emits %q, which is not a qdrant.ConfigSpec json tag", key)
		}
	}

	for tag := range tags {
		if !emitted[tag] {
			t.Errorf("qdrant.ConfigSpec has %q but expand does not emit it", tag)
		}
	}
}
