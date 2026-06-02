package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// packageSpecGuard mirrors the json tags of cozystack.io/v1alpha1 PackageSpec.
// It is vendored (not imported) because that type lives in the heavy root module.
type packageSpecGuard struct {
	Variant            string         `json:"variant,omitempty"`
	IgnoreDependencies []string       `json:"ignoreDependencies,omitempty"`
	Components         map[string]any `json:"components,omitempty"`
}

func fullPackageModel() packageModel {
	return packageModel{
		Name:    types.StringValue("cozystack.bucket-application"),
		Variant: types.StringValue("default"),
		IgnoreDependencies: types.ListValueMust(types.StringType, []attr.Value{
			types.StringValue("cozystack.networking"),
		}),
		Components: jsontypes.NewNormalizedValue(`{"bucket-app":{"enabled":false,"values":{"replicas":2}}}`),
	}
}

func TestPackageExpand_VariantAndComponents(t *testing.T) {
	t.Parallel()

	model := fullPackageModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec["variant"] != "default" {
		t.Errorf("variant = %v, want default", got.Spec["variant"])
	}

	components, _ := got.Spec["components"].(map[string]any)

	app, _ := components["bucket-app"].(map[string]any)
	if app["enabled"] != false {
		t.Errorf("components.bucket-app.enabled = %v, want false", app["enabled"])
	}
}

func TestPackageFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name: "cozystack.bucket-application",
		Spec: map[string]any{
			"variant":            "default",
			"ignoreDependencies": []any{"cozystack.networking"},
			"components":         map[string]any{"bucket-app": map[string]any{"enabled": false}},
		},
		UID:    "abc-123",
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model packageModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "cozystack.bucket-application" {
		t.Errorf("id = %q, want the name (cluster-scoped)", model.ID.ValueString())
	}

	if model.UID.ValueString() != "abc-123" {
		t.Errorf("uid = %q, want abc-123", model.UID.ValueString())
	}

	if model.Components.IsNull() {
		t.Errorf("components is null, want populated JSON")
	}
}

func TestPackageExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullPackageModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, packageSpecGuard{})
}
