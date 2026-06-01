package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/vpn"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

func fullVPNModel() vpnModel {
	usersType := types.ObjectType{AttrTypes: passwordUserObjectType()}

	return vpnModel{
		Name:            types.StringValue("gw"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(2),
		Resources:       types.ObjectNull(resourcesObjectType()),
		ResourcesPreset: types.StringValue("t1.nano"),
		External:        types.BoolValue(true),
		Host:            types.StringValue("vpn.example.test"),
		Users: types.MapValueMust(usersType, map[string]attr.Value{
			"alice": types.ObjectValueMust(passwordUserObjectType(), map[string]attr.Value{
				"password": types.StringValue("secret"),
			}),
		}),
		ExternalIPs: types.ListValueMust(types.StringType, []attr.Value{
			types.StringValue("203.0.113.7"),
		}),
	}
}

func TestVPNExpand_ListAndUsers(t *testing.T) {
	t.Parallel()

	model := fullVPNModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	ips, ok := got.Spec["externalIPs"].([]any)
	if !ok || len(ips) != 1 || ips[0] != "203.0.113.7" {
		t.Errorf("spec.externalIPs = %v, want [203.0.113.7]", got.Spec["externalIPs"])
	}

	users, ok := got.Spec["users"].(map[string]any)
	if !ok {
		t.Fatalf("spec.users type = %T", got.Spec["users"])
	}

	alice, _ := users["alice"].(map[string]any)
	if alice["password"] != "secret" {
		t.Errorf("users.alice.password = %v, want secret", alice["password"])
	}
}

func TestVPNFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "gw",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(2),
			"resourcesPreset": "t1.nano",
			"external":        false,
			"host":            "",
			"resources":       map[string]any{},
			"users":           map[string]any{"bob": map[string]any{"password": "p"}},
			"externalIPs":     []any{"203.0.113.9"},
		},
		Status: client.ApplicationStatus{Version: "1.1.0", Ready: true},
	}

	var model vpnModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ExternalIPs.IsNull() {
		t.Errorf("externalIPs should not be null")
	}
	if _, ok := model.Users.Elements()["bob"]; !ok {
		t.Errorf("users missing bob")
	}
}

func TestVPNExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullVPNModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	tags := configSpecJSONTags(vpn.ConfigSpec{})

	for key := range emitted {
		if !tags[key] {
			t.Errorf("expand emits %q, not a vpn.ConfigSpec json tag", key)
		}
	}

	for tag := range tags {
		if !emitted[tag] {
			t.Errorf("vpn.ConfigSpec has %q but expand does not emit it", tag)
		}
	}
}
