package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/harbor"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullHarborModel() harborModel {
	return harborModel{
		Name:         types.StringValue("registry"),
		Namespace:    types.StringValue("tenant-root"),
		Host:         types.StringValue("registry.example.com"),
		StorageClass: types.StringValue(""),
	}
}

func TestHarborExpand_OmitsEmptyHost(t *testing.T) {
	t.Parallel()

	model := fullHarborModel()
	model.Host = types.StringValue("")

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if _, ok := got.Spec[attrHost]; ok {
		t.Errorf("host key present for empty host, want omitted")
	}
}

func TestHarborExpand_Host(t *testing.T) {
	t.Parallel()

	model := fullHarborModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec[attrHost] != "registry.example.com" {
		t.Errorf("host = %v, want registry.example.com", got.Spec[attrHost])
	}
}

func TestHarborFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "registry",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"host":         "registry.example.com",
			"storageClass": "fast",
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model harborModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Host.ValueString() != "registry.example.com" {
		t.Errorf("host = %q, want registry.example.com", model.Host.ValueString())
	}

	if model.StorageClass.ValueString() != "fast" {
		t.Errorf("storage_class = %q, want fast", model.StorageClass.ValueString())
	}
}

func TestHarborExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullHarborModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, harbor.ConfigSpec{},
		"core", "registry", "jobservice", "trivy", "database", "redis")
}
