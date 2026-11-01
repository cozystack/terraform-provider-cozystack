package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/openbao"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullOpenbaoModel() openbaoModel {
	return openbaoModel{
		Name:            types.StringValue("vault"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(1),
		Size:            types.StringValue("10Gi"),
		StorageClass:    types.StringValue(""),
		External:        types.BoolValue(false),
		UI:              types.BoolValue(true),
		ResourcesPreset: types.StringValue("t1.small"),
		Resources:       resourcesBlock(),
	}
}

func TestOpenbaoExpand_CoreFields(t *testing.T) {
	t.Parallel()

	model := fullOpenbaoModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec["ui"] != true {
		t.Errorf("spec.ui = %v, want true", got.Spec["ui"])
	}
	if got.Spec["replicas"] != int64(1) {
		t.Errorf("spec.replicas = %v, want 1", got.Spec["replicas"])
	}
}

func TestOpenbaoFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "vault",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(1),
			"size":            "10Gi",
			"storageClass":    "",
			"external":        false,
			"ui":              false,
			"resourcesPreset": "t1.small",
			"resources":       map[string]any{},
		},
		Status: client.ApplicationStatus{Version: "2.0.0", Ready: true},
	}

	var model openbaoModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.UI.ValueBool() {
		t.Errorf("ui = true, want false")
	}
	if model.ID.ValueString() != "tenant-root/vault" {
		t.Errorf("id = %q, want tenant-root/vault", model.ID.ValueString())
	}
}

func TestOpenbaoExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullOpenbaoModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	assertSpecCoverage(t, got.Spec, openbao.ConfigSpec{})
}
