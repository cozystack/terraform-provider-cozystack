package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/httpcache"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

func fullHTTPCacheModel() httpcacheModel {
	return httpcacheModel{
		Name:         types.StringValue("cache"),
		Namespace:    types.StringValue("tenant-root"),
		Size:         types.StringValue("10Gi"),
		StorageClass: types.StringValue(""),
		External:     types.BoolValue(false),
		Endpoints: types.ListValueMust(types.StringType, []attr.Value{
			types.StringValue("192.0.2.10:80"),
		}),
	}
}

func TestHTTPCacheExpand_Endpoints(t *testing.T) {
	t.Parallel()

	model := fullHTTPCacheModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	endpoints, _ := got.Spec["endpoints"].([]any)
	if len(endpoints) != 1 || endpoints[0] != "192.0.2.10:80" {
		t.Errorf("endpoints = %v, want [192.0.2.10:80]", endpoints)
	}
}

func TestHTTPCacheFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "cache",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"size":         "20Gi",
			"storageClass": "fast",
			"external":     true,
			"endpoints":    []any{"192.0.2.11:8080"},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model httpcacheModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Size.ValueString() != "20Gi" {
		t.Errorf("size = %q, want 20Gi", model.Size.ValueString())
	}

	if !model.External.ValueBool() {
		t.Errorf("external = false, want true")
	}

	if model.Endpoints.Elements()[0].(types.String).ValueString() != "192.0.2.11:8080" {
		t.Errorf("endpoints = %v, want [192.0.2.11:8080]", model.Endpoints.Elements())
	}
}

func TestHTTPCacheExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullHTTPCacheModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, httpcache.ConfigSpec{}, "haproxy", "nginx")
}
