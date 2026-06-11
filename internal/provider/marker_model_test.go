package provider

import (
	"context"
	"testing"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMarkerModel_RoundTrip(t *testing.T) {
	t.Parallel()

	model := markerModel{Name: types.StringValue("tenant-team")}

	app, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if app.Spec != nil {
		t.Errorf("marker spec = %v, want nil (no spec)", app.Spec)
	}

	out := &client.Application{Name: "tenant-team", UID: "uid-1", Status: client.ApplicationStatus{Ready: true}}
	if diags := model.flatten(out); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "tenant-team" || model.UID.ValueString() != "uid-1" {
		t.Errorf("id/uid = %q/%q, want tenant-team/uid-1 (cluster marker)", model.ID.ValueString(), model.UID.ValueString())
	}
}

func TestMarkerNsModel_RoundTrip(t *testing.T) {
	t.Parallel()

	model := markerNsModel{Name: types.StringValue("etcd"), Namespace: types.StringValue("tenant-root")}

	app, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if app.Spec != nil || app.Namespace != "tenant-root" {
		t.Errorf("expand = spec %v ns %q, want nil spec and tenant-root", app.Spec, app.Namespace)
	}

	out := &client.Application{Name: "etcd", Namespace: "tenant-root", UID: "uid-2"}
	if diags := model.flatten(out); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "tenant-root/etcd" {
		t.Errorf("id = %q, want tenant-root/etcd", model.ID.ValueString())
	}
}
