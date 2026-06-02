package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// opensearchModel maps the cozystack_opensearch schema to Go types. The images,
// nodeRoles, and dashboards blocks are not managed (they use server defaults).
type opensearchModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Namespace            types.String `tfsdk:"namespace"`
	Replicas             types.Int64  `tfsdk:"replicas"`
	Resources            types.Object `tfsdk:"resources"`
	ResourcesPreset      types.String `tfsdk:"resources_preset"`
	Size                 types.String `tfsdk:"size"`
	StorageClass         types.String `tfsdk:"storage_class"`
	External             types.Bool   `tfsdk:"external"`
	TopologySpreadPolicy types.String `tfsdk:"topology_spread_policy"`
	Version              types.String `tfsdk:"version"`
	Users                types.Map    `tfsdk:"users"`
	Ready                types.Bool   `tfsdk:"ready"`
	ChartVersion         types.String `tfsdk:"chart_version"`
	UID                  types.String `tfsdk:"uid"`
}

type opensearchResourceModel struct {
	opensearchModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *opensearchResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *opensearchModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func opensearchUserObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"password": types.StringType,
		"roles":    types.ListType{ElemType: types.StringType},
	}
}

type opensearchUserData struct {
	Password types.String `tfsdk:"password"`
	Roles    []string     `tfsdk:"roles"`
}

func expandOpensearchUsers(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]opensearchUserData{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, user := range elements {
		entry := map[string]any{}
		if password := user.Password.ValueString(); password != "" {
			entry["password"] = password
		}

		if len(user.Roles) > 0 {
			entry["roles"] = stringsToAny(user.Roles)
		}

		out[name] = entry
	}

	return out, diags
}

func flattenOpensearchUsers(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, opensearchUserObjectType(), func(user map[string]any) map[string]attr.Value {
		password, _ := user["password"].(string)

		return map[string]attr.Value{
			"password": types.StringValue(password),
			"roles":    stringListOrNull(user["roles"]),
		}
	})
}

func (m *opensearchModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	resources, rDiags := expandResources(ctx, m.Resources)
	diags.Append(rDiags...)

	users, uDiags := expandOpensearchUsers(ctx, m.Users)
	diags.Append(uDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrReplicas:           m.Replicas.ValueInt64(),
		attrResources:          resources,
		specResourcesPreset:    m.ResourcesPreset.ValueString(),
		attrSize:               m.Size.ValueString(),
		specStorageClass:       m.StorageClass.ValueString(),
		attrExternal:           m.External.ValueBool(),
		"topologySpreadPolicy": m.TopologySpreadPolicy.ValueString(),
		attrVersion:            m.Version.ValueString(),
		"users":                users,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func (m *opensearchModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Replicas = types.Int64Value(specInt64(app.Spec, attrReplicas))
	m.ResourcesPreset = types.StringValue(specString(app.Spec, specResourcesPreset))
	m.Size = types.StringValue(specString(app.Spec, attrSize))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.External = types.BoolValue(specBool(app.Spec, attrExternal))
	m.TopologySpreadPolicy = types.StringValue(specString(app.Spec, "topologySpreadPolicy"))
	m.Version = types.StringValue(specString(app.Spec, attrVersion))

	resources, rDiags := flattenResources(app.Spec[attrResources])
	diags.Append(rDiags...)

	m.Resources = resources

	users, uDiags := flattenOpensearchUsers(app.Spec["users"])
	diags.Append(uDiags...)

	m.Users = users

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}
