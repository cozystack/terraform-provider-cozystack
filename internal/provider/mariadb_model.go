package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// mariadbModel maps the cozystack_mariadb schema to Go types. The deprecated
// inline `backup` block is intentionally not managed (use the BackupClass flow).
type mariadbModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Namespace       types.String `tfsdk:"namespace"`
	Replicas        types.Int64  `tfsdk:"replicas"`
	Resources       types.Object `tfsdk:"resources"`
	ResourcesPreset types.String `tfsdk:"resources_preset"`
	Size            types.String `tfsdk:"size"`
	StorageClass    types.String `tfsdk:"storage_class"`
	External        types.Bool   `tfsdk:"external"`
	Version         types.String `tfsdk:"version"`
	Users           types.Map    `tfsdk:"users"`
	Databases       types.Map    `tfsdk:"databases"`
	Ready           types.Bool   `tfsdk:"ready"`
	ChartVersion    types.String `tfsdk:"chart_version"`
	UID             types.String `tfsdk:"uid"`
}

type mariadbResourceModel struct {
	mariadbModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *mariadbResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *mariadbModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func mariadbUserObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"max_user_connections": types.Int64Type,
		"password":             types.StringType,
	}
}

type mariadbUserData struct {
	MaxUserConnections types.Int64  `tfsdk:"max_user_connections"`
	Password           types.String `tfsdk:"password"`
}

func expandMariadbUsers(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]mariadbUserData{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, user := range elements {
		entry := map[string]any{"maxUserConnections": user.MaxUserConnections.ValueInt64()}
		if password := user.Password.ValueString(); password != "" {
			entry["password"] = password
		}

		out[name] = entry
	}

	return out, diags
}

func flattenMariadbUsers(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, mariadbUserObjectType(), func(user map[string]any) map[string]attr.Value {
		password, _ := user["password"].(string)

		return map[string]attr.Value{
			"max_user_connections": types.Int64Value(anyToInt64(user["maxUserConnections"])),
			"password":             types.StringValue(password),
		}
	})
}

func anyToInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func (m *mariadbModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	resources, rDiags := expandResources(ctx, m.Resources)
	diags.Append(rDiags...)

	users, uDiags := expandMariadbUsers(ctx, m.Users)
	diags.Append(uDiags...)

	databases, dDiags := expandRolesMap(ctx, m.Databases)
	diags.Append(dDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrReplicas:        m.Replicas.ValueInt64(),
		attrResources:       resources,
		specResourcesPreset: m.ResourcesPreset.ValueString(),
		attrSize:            m.Size.ValueString(),
		specStorageClass:    m.StorageClass.ValueString(),
		attrExternal:        m.External.ValueBool(),
		attrVersion:         m.Version.ValueString(),
		"users":             users,
		"databases":         databases,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func (m *mariadbModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Replicas = types.Int64Value(specInt64(app.Spec, attrReplicas))
	m.ResourcesPreset = types.StringValue(specString(app.Spec, specResourcesPreset))
	m.Size = types.StringValue(specString(app.Spec, attrSize))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.External = types.BoolValue(specBool(app.Spec, attrExternal))
	m.Version = types.StringValue(specString(app.Spec, attrVersion))

	resources, rDiags := flattenResources(app.Spec[attrResources])
	diags.Append(rDiags...)

	m.Resources = resources

	users, uDiags := flattenMariadbUsers(app.Spec["users"])
	diags.Append(uDiags...)

	m.Users = users

	databases, dDiags := flattenRolesMap(app.Spec["databases"])
	diags.Append(dDiags...)

	m.Databases = databases

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}
