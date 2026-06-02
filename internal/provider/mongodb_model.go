package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// mongodbModel maps the cozystack_mongodb schema to Go types. The deprecated
// inline backup, the bootstrap restore block, and the detailed shardingConfig
// are intentionally not managed (toggle sharding with `sharding`).
type mongodbModel struct {
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
	Sharding        types.Bool   `tfsdk:"sharding"`
	Users           types.Map    `tfsdk:"users"`
	Databases       types.Map    `tfsdk:"databases"`
	Ready           types.Bool   `tfsdk:"ready"`
	ChartVersion    types.String `tfsdk:"chart_version"`
	UID             types.String `tfsdk:"uid"`
}

type mongodbResourceModel struct {
	mongodbModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *mongodbResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *mongodbModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func (m *mongodbModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	resources, rDiags := expandResources(ctx, m.Resources)
	diags.Append(rDiags...)

	users, uDiags := expandPasswordUsers(ctx, m.Users)
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
		"sharding":          m.Sharding.ValueBool(),
		"users":             users,
		"databases":         databases,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func (m *mongodbModel) flatten(app *client.Application) diag.Diagnostics {
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
	m.Sharding = types.BoolValue(specBool(app.Spec, "sharding"))

	resources, rDiags := flattenResources(app.Spec[attrResources])
	diags.Append(rDiags...)

	m.Resources = resources

	users, uDiags := flattenPasswordUsers(app.Spec["users"])
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
