package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/tcpbalancer"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullTCPBalancerModel() tcpbalancerModel {
	return tcpbalancerModel{
		Name:            types.StringValue("lb"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(2),
		Resources:       types.ObjectNull(resourcesObjectType()),
		ResourcesPreset: types.StringValue("t1.nano"),
		External:        types.BoolValue(false),
		WhitelistHTTP:   types.BoolValue(true),
		Whitelist: types.ListValueMust(types.StringType, []attr.Value{
			types.StringValue("192.0.2.0/24"),
		}),
	}
}

func TestTCPBalancerExpand_Whitelist(t *testing.T) {
	t.Parallel()

	model := fullTCPBalancerModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec["whitelistHTTP"] != true {
		t.Errorf("whitelistHTTP = %v, want true", got.Spec["whitelistHTTP"])
	}

	whitelist, _ := got.Spec["whitelist"].([]any)
	if len(whitelist) != 1 || whitelist[0] != "192.0.2.0/24" {
		t.Errorf("whitelist = %v, want [192.0.2.0/24]", whitelist)
	}
}

func TestTCPBalancerFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "lb",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(3),
			"resourcesPreset": "t1.micro",
			"external":        true,
			"whitelistHTTP":   true,
			"whitelist":       []any{"198.51.100.0/24"},
			"resources":       map[string]any{},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model tcpbalancerModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Replicas.ValueInt64() != 3 {
		t.Errorf("replicas = %d, want 3", model.Replicas.ValueInt64())
	}

	if !model.WhitelistHTTP.ValueBool() {
		t.Errorf("whitelist_http = false, want true")
	}
}

func TestTCPBalancerExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullTCPBalancerModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, tcpbalancer.ConfigSpec{}, "httpAndHttps")
}
