package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

func TestRawSpecExpand_ParsesJSON(t *testing.T) {
	t.Parallel()

	model := rawSpecModel{
		Name: types.StringValue("custom-source"),
		Spec: jsontypes.NewNormalizedValue(`{"sourceRef":{"kind":"OCIRepository","name":"repo","namespace":"cozy-system"}}`),
	}

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	sourceRef, _ := got.Spec["sourceRef"].(map[string]any)
	if sourceRef["kind"] != "OCIRepository" {
		t.Errorf("sourceRef.kind = %v, want OCIRepository", sourceRef["kind"])
	}
}

func TestRawSpecFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:   "custom-source",
		Spec:   map[string]any{"sourceRef": map[string]any{"kind": "GitRepository", "name": "r"}},
		UID:    "uid-9",
		Status: client.ApplicationStatus{Version: "2.0.0", Ready: true},
	}

	var model rawSpecModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "custom-source" {
		t.Errorf("id = %q, want custom-source", model.ID.ValueString())
	}

	if model.Spec.IsNull() {
		t.Errorf("spec is null, want populated JSON")
	}

	if model.UID.ValueString() != "uid-9" {
		t.Errorf("uid = %q, want uid-9", model.UID.ValueString())
	}
}
