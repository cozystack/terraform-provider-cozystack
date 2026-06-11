package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/vmdisk"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullVMDiskModel() vmdiskModel {
	source := types.ObjectValueMust(vmdiskSourceObjectType(), map[string]attr.Value{
		"disk": types.ObjectNull(vmdiskNameObjectType()),
		"http": types.ObjectValueMust(vmdiskURLObjectType(), map[string]attr.Value{
			"url": types.StringValue("https://example.com/cirros.img"),
		}),
		"image": types.ObjectNull(vmdiskNameObjectType()),
	})

	return vmdiskModel{
		Name:         types.StringValue("disk"),
		Namespace:    types.StringValue("tenant-root"),
		Source:       source,
		Optical:      types.BoolValue(false),
		Storage:      types.StringValue("5Gi"),
		StorageClass: types.StringValue("replicated"),
	}
}

func TestVMDiskExpand_Source(t *testing.T) {
	t.Parallel()

	model := fullVMDiskModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	source, _ := got.Spec["source"].(map[string]any)
	if _, ok := source["disk"]; ok {
		t.Errorf("source has disk for an http-only model: %v", source)
	}

	httpSource, _ := source["http"].(map[string]any)
	if httpSource["url"] != "https://example.com/cirros.img" {
		t.Errorf("source.http.url = %v, want example URL", httpSource["url"])
	}
}

func TestVMDiskFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "disk",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"source":       map[string]any{"image": map[string]any{"name": "ubuntu"}},
			"optical":      false,
			"storage":      "10Gi",
			"storageClass": "replicated",
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model vmdiskModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Storage.ValueString() != "10Gi" {
		t.Errorf("storage = %q, want 10Gi", model.Storage.ValueString())
	}

	source := model.Source.Attributes()
	if source["image"].IsNull() {
		t.Errorf("source.image is null, want populated")
	}

	if !source["http"].IsNull() {
		t.Errorf("source.http populated, want null")
	}
}

func TestVMDiskExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullVMDiskModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, vmdisk.ConfigSpec{})
}
