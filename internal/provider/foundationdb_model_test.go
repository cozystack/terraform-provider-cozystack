package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/foundationdb"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

func fullFoundationDBModel() foundationdbModel {
	return foundationdbModel{
		Name:                  types.StringValue("fdb"),
		Namespace:             types.StringValue("tenant-root"),
		Storage:               fdbStorageDefault(),
		Resources:             types.ObjectNull(resourcesObjectType()),
		ResourcesPreset:       types.StringValue("c1.small"),
		CustomParameters:      types.ListNull(types.StringType),
		ImageType:             types.StringValue("unified"),
		AutomaticReplacements: types.BoolValue(true),
	}
}

func TestFoundationDBExpand_Storage(t *testing.T) {
	t.Parallel()

	model := fullFoundationDBModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	storage, _ := got.Spec["storage"].(map[string]any)
	if storage["size"] != "16Gi" {
		t.Errorf("storage.size = %v, want 16Gi", storage["size"])
	}

	if got.Spec["imageType"] != "unified" {
		t.Errorf("imageType = %v, want unified", got.Spec["imageType"])
	}
}

func TestFoundationDBFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "fdb",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"storage":               map[string]any{"size": "32Gi", "storageClass": "fast"},
			"resources":             map[string]any{},
			"resourcesPreset":       "c1.medium",
			"customParameters":      []any{"knob_disable_posix_kernel_aio=1"},
			"imageType":             "split",
			"automaticReplacements": false,
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model foundationdbModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ImageType.ValueString() != "split" {
		t.Errorf("image_type = %q, want split", model.ImageType.ValueString())
	}

	storage := model.Storage.Attributes()
	if storage["size"].(types.String).ValueString() != "32Gi" {
		t.Errorf("storage.size = %v, want 32Gi", storage["size"])
	}

	if len(model.CustomParameters.Elements()) != 1 {
		t.Errorf("custom_parameters = %v, want one element", model.CustomParameters.Elements())
	}
}

func TestFoundationDBExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullFoundationDBModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, foundationdb.ConfigSpec{},
		"cluster", "backup", "monitoring", "securityContext")
}
