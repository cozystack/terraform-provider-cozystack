package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/bucket"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func bucketUsersValue(t *testing.T, users map[string]bool) types.Map {
	t.Helper()

	elementType := types.ObjectType{AttrTypes: bucketUserObjectType()}
	elements := make(map[string]attr.Value, len(users))

	for name, readonly := range users {
		elements[name] = types.ObjectValueMust(bucketUserObjectType(), map[string]attr.Value{
			"readonly": types.BoolValue(readonly),
		})
	}

	return types.MapValueMust(elementType, elements)
}

func TestBucketExpand_Users(t *testing.T) {
	t.Parallel()

	model := bucketModel{
		Name:        types.StringValue("assets"),
		Namespace:   types.StringValue("tenant-root"),
		Locking:     types.BoolValue(true),
		StoragePool: types.StringValue(""),
		Users:       bucketUsersValue(t, map[string]bool{"reader": true, "writer": false}),
	}

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec["locking"] != true {
		t.Errorf("spec.locking = %v, want true", got.Spec["locking"])
	}

	users, ok := got.Spec["users"].(map[string]any)
	if !ok {
		t.Fatalf("spec.users type = %T, want map", got.Spec["users"])
	}

	reader, _ := users["reader"].(map[string]any)
	if reader["readonly"] != true {
		t.Errorf("users.reader.readonly = %v, want true", reader["readonly"])
	}

	writer, _ := users["writer"].(map[string]any)
	if writer["readonly"] != false {
		t.Errorf("users.writer.readonly = %v, want false", writer["readonly"])
	}
}

func TestBucketFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "assets",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"locking":     false,
			"storagePool": "pool-a",
			"users":       map[string]any{"reader": map[string]any{"readonly": true}},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model bucketModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.StoragePool.ValueString() != "pool-a" {
		t.Errorf("storage_pool = %q, want pool-a", model.StoragePool.ValueString())
	}

	users := model.Users.Elements()
	if _, ok := users["reader"]; !ok {
		t.Errorf("users missing reader: %v", users)
	}
}

func TestBucketFlatten_EmptyUsersIsNull(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "assets",
		Namespace: "tenant-root",
		Spec:      map[string]any{"users": map[string]any{}},
	}

	var model bucketModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !model.Users.IsNull() {
		t.Errorf("empty users should flatten to null")
	}
}

// TestBucketExpandKeysMatchConfigSpec guards against field-name drift.
func TestBucketExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := bucketModel{
		Name:      types.StringValue("assets"),
		Namespace: types.StringValue("tenant-root"),
		Users:     types.MapNull(types.ObjectType{AttrTypes: bucketUserObjectType()}),
	}

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	tags := configSpecJSONTags(bucket.ConfigSpec{})

	for key := range emitted {
		if !tags[key] {
			t.Errorf("expand emits %q, which is not a bucket.ConfigSpec json tag", key)
		}
	}

	for tag := range tags {
		if !emitted[tag] {
			t.Errorf("bucket.ConfigSpec has %q but expand does not emit it", tag)
		}
	}
}
