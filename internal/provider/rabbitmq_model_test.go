package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/rabbitmq"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullRabbitmqModel() rabbitmqModel {
	usersType := types.ObjectType{AttrTypes: passwordUserObjectType()}
	vhostsType := types.ObjectType{AttrTypes: rolesEntryObjectType()}

	rolesObj := types.ObjectValueMust(rolesObjectType(), map[string]attr.Value{
		"admin":    types.ListValueMust(types.StringType, []attr.Value{types.StringValue("alice")}),
		"readonly": types.ListNull(types.StringType),
	})

	return rabbitmqModel{
		Name:            types.StringValue("queue"),
		Namespace:       types.StringValue("tenant-root"),
		Replicas:        types.Int64Value(3),
		Resources:       types.ObjectNull(resourcesObjectType()),
		ResourcesPreset: types.StringValue("s1.nano"),
		Size:            types.StringValue("10Gi"),
		StorageClass:    types.StringValue(""),
		External:        types.BoolValue(false),
		Version:         types.StringValue("v4.2"),
		Users: types.MapValueMust(usersType, map[string]attr.Value{
			"alice": types.ObjectValueMust(passwordUserObjectType(), map[string]attr.Value{"password": types.StringValue("p")}),
		}),
		Vhosts: types.MapValueMust(vhostsType, map[string]attr.Value{
			"main": types.ObjectValueMust(rolesEntryObjectType(), map[string]attr.Value{"roles": rolesObj}),
		}),
	}
}

func TestRabbitmqExpand_UsersAndVhosts(t *testing.T) {
	t.Parallel()

	model := fullRabbitmqModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}
	if got.Spec[specResourcesPreset] != "s1.nano" {
		t.Errorf("resourcesPreset = %v, want s1.nano", got.Spec[specResourcesPreset])
	}

	users, _ := got.Spec["users"].(map[string]any)
	alice, _ := users["alice"].(map[string]any)
	if alice["password"] != "p" {
		t.Errorf("users.alice.password = %v, want p", alice["password"])
	}

	vhosts, _ := got.Spec["vhosts"].(map[string]any)
	main, _ := vhosts["main"].(map[string]any)
	roles, _ := main["roles"].(map[string]any)

	admin, ok := roles["admin"].([]any)
	if !ok || len(admin) != 1 || admin[0] != "alice" {
		t.Errorf("vhosts.main.roles.admin = %v, want [alice]", roles["admin"])
	}
}

func TestRabbitmqFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "queue",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"replicas":        int64(3),
			"resourcesPreset": "s1.nano",
			"size":            "10Gi",
			"storageClass":    "",
			"external":        false,
			"version":         "v4.1",
			"resources":       map[string]any{},
			"users":           map[string]any{"bob": map[string]any{"password": "x"}},
			"vhosts":          map[string]any{"v": map[string]any{"roles": map[string]any{"admin": []any{"bob"}}}},
		},
		Status: client.ApplicationStatus{Version: "2.0.0", Ready: true},
	}

	var model rabbitmqModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Version.ValueString() != "v4.1" {
		t.Errorf("version = %q, want v4.1", model.Version.ValueString())
	}
	if model.ResourcesPreset.ValueString() != "s1.nano" {
		t.Errorf("resourcesPreset = %q, want s1.nano", model.ResourcesPreset.ValueString())
	}
	if _, ok := model.Vhosts.Elements()["v"]; !ok {
		t.Errorf("vhosts missing v")
	}
}

func TestRabbitmqExpand_ExplicitLegacyPreset(t *testing.T) {
	t.Parallel()

	model := fullRabbitmqModel()
	model.ResourcesPreset = types.StringValue("t1.nano")

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}
	if got.Spec[specResourcesPreset] != "t1.nano" {
		t.Errorf("resourcesPreset = %v, want t1.nano", got.Spec[specResourcesPreset])
	}
}

func TestRabbitmqExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullRabbitmqModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	tags := configSpecJSONTags(rabbitmq.ConfigSpec{})

	for key := range emitted {
		if !tags[key] {
			t.Errorf("expand emits %q, not a rabbitmq.ConfigSpec json tag", key)
		}
	}

	for tag := range tags {
		if !emitted[tag] {
			t.Errorf("rabbitmq.ConfigSpec has %q but expand does not emit it", tag)
		}
	}
}
