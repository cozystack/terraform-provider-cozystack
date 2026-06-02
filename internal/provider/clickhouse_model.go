package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// clickhouseModel maps the cozystack_clickhouse schema to Go types. The
// deprecated backup block and the clickhouseKeeper block are not managed.
type clickhouseModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Namespace       types.String `tfsdk:"namespace"`
	Replicas        types.Int64  `tfsdk:"replicas"`
	Shards          types.Int64  `tfsdk:"shards"`
	Resources       types.Object `tfsdk:"resources"`
	ResourcesPreset types.String `tfsdk:"resources_preset"`
	Size            types.String `tfsdk:"size"`
	StorageClass    types.String `tfsdk:"storage_class"`
	LogStorageSize  types.String `tfsdk:"log_storage_size"`
	LogTTL          types.Int64  `tfsdk:"log_ttl"`
	Users           types.Map    `tfsdk:"users"`
	Ready           types.Bool   `tfsdk:"ready"`
	ChartVersion    types.String `tfsdk:"chart_version"`
	UID             types.String `tfsdk:"uid"`
}

type clickhouseResourceModel struct {
	clickhouseModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *clickhouseResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *clickhouseModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func clickhouseUserObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"password": types.StringType,
		"readonly": types.BoolType,
	}
}

type clickhouseUserData struct {
	Password types.String `tfsdk:"password"`
	Readonly types.Bool   `tfsdk:"readonly"`
}

func expandClickhouseUsers(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]clickhouseUserData{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, user := range elements {
		entry := map[string]any{"readonly": user.Readonly.ValueBool()}
		if password := user.Password.ValueString(); password != "" {
			entry["password"] = password
		}

		out[name] = entry
	}

	return out, diags
}

func flattenClickhouseUsers(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, clickhouseUserObjectType(), func(user map[string]any) map[string]attr.Value {
		password, _ := user["password"].(string)
		readonly, _ := user["readonly"].(bool)

		return map[string]attr.Value{
			"password": types.StringValue(password),
			"readonly": types.BoolValue(readonly),
		}
	})
}

func (m *clickhouseModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	resources, rDiags := expandResources(ctx, m.Resources)
	diags.Append(rDiags...)

	users, uDiags := expandClickhouseUsers(ctx, m.Users)
	diags.Append(uDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrReplicas:        m.Replicas.ValueInt64(),
		"shards":            m.Shards.ValueInt64(),
		attrResources:       resources,
		specResourcesPreset: m.ResourcesPreset.ValueString(),
		attrSize:            m.Size.ValueString(),
		specStorageClass:    m.StorageClass.ValueString(),
		"logStorageSize":    m.LogStorageSize.ValueString(),
		"logTTL":            m.LogTTL.ValueInt64(),
		"users":             users,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func (m *clickhouseModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Replicas = types.Int64Value(specInt64(app.Spec, attrReplicas))
	m.Shards = types.Int64Value(specInt64(app.Spec, "shards"))
	m.ResourcesPreset = types.StringValue(specString(app.Spec, specResourcesPreset))
	m.Size = types.StringValue(specString(app.Spec, attrSize))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.LogStorageSize = types.StringValue(specString(app.Spec, "logStorageSize"))
	m.LogTTL = types.Int64Value(specInt64(app.Spec, "logTTL"))

	resources, rDiags := flattenResources(app.Spec[attrResources])
	diags.Append(rDiags...)

	m.Resources = resources

	users, uDiags := flattenClickhouseUsers(app.Spec["users"])
	diags.Append(uDiags...)

	m.Users = users

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}
