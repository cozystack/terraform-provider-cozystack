package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// bucketModel maps the cozystack_bucket schema to Go types. The spec attributes
// mirror the json tags of the pinned bucket.ConfigSpec one-to-one.
type bucketModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Locking      types.Bool   `tfsdk:"locking"`
	StoragePool  types.String `tfsdk:"storage_pool"`
	Users        types.Map    `tfsdk:"users"`
	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
}

type bucketUserModel struct {
	Readonly types.Bool `tfsdk:"readonly"`
}

func bucketUserObjectType() map[string]attr.Type {
	return map[string]attr.Type{"readonly": types.BoolType}
}

// identity returns the bucket's namespace and name.
func (m *bucketModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

// expand converts the Terraform model into a client.Application ready to send.
func (m *bucketModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	users, uDiags := expandBucketUsers(ctx, m.Users)
	diags.Append(uDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		"locking":     m.Locking.ValueBool(),
		"storagePool": m.StoragePool.ValueString(),
		"users":       users,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

// flatten populates the Terraform model from the server view of a bucket.
func (m *bucketModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Locking = types.BoolValue(specBool(app.Spec, "locking"))
	m.StoragePool = types.StringValue(specString(app.Spec, "storagePool"))

	users, uDiags := flattenBucketUsers(app.Spec["users"])
	diags.Append(uDiags...)

	m.Users = users

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}

// expandBucketUsers renders the users map into a spec submap.
func expandBucketUsers(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]bucketUserModel{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, user := range elements {
		out[name] = map[string]any{"readonly": user.Readonly.ValueBool()}
	}

	return out, diags
}

// flattenBucketUsers builds the users map from a spec submap. An empty or absent
// map flattens to null so an unset block does not drift.
func flattenBucketUsers(raw any) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics

	elementType := types.ObjectType{AttrTypes: bucketUserObjectType()}

	users, ok := raw.(map[string]any)
	if !ok || len(users) == 0 {
		return types.MapNull(elementType), diags
	}

	elements := make(map[string]attr.Value, len(users))

	for name, raw := range users {
		user, _ := raw.(map[string]any)
		readonly, _ := user["readonly"].(bool)

		object, objectDiags := types.ObjectValue(bucketUserObjectType(), map[string]attr.Value{
			"readonly": types.BoolValue(readonly),
		})
		diags.Append(objectDiags...)

		elements[name] = object
	}

	value, mapDiags := types.MapValue(elementType, elements)
	diags.Append(mapDiags...)

	return value, diags
}
