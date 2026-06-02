package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// redisModel maps the cozystack_redis schema to Go types. The spec attributes
// mirror the json tags of the pinned redis.ConfigSpec one-to-one.
type redisModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Namespace       types.String `tfsdk:"namespace"`
	Replicas        types.Int64  `tfsdk:"replicas"`
	Size            types.String `tfsdk:"size"`
	StorageClass    types.String `tfsdk:"storage_class"`
	External        types.Bool   `tfsdk:"external"`
	Version         types.String `tfsdk:"version"`
	AuthEnabled     types.Bool   `tfsdk:"auth_enabled"`
	ResourcesPreset types.String `tfsdk:"resources_preset"`
	Resources       types.Object `tfsdk:"resources"`
	Ready           types.Bool   `tfsdk:"ready"`
	ChartVersion    types.String `tfsdk:"chart_version"`
	UID             types.String `tfsdk:"uid"`
}

// identity returns the redis instance's namespace and name.
func (m *redisModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

// expand converts the Terraform model into a client.Application ready to send.
func (m *redisModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	resources, rDiags := expandResources(ctx, m.Resources)
	diags.Append(rDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrReplicas:        m.Replicas.ValueInt64(),
		attrSize:            m.Size.ValueString(),
		specStorageClass:    m.StorageClass.ValueString(),
		attrExternal:        m.External.ValueBool(),
		attrVersion:         m.Version.ValueString(),
		specAuthEnabled:     m.AuthEnabled.ValueBool(),
		specResourcesPreset: m.ResourcesPreset.ValueString(),
		attrResources:       resources,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

// flatten populates the Terraform model from the server view of a redis.
func (m *redisModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Replicas = types.Int64Value(specInt64(app.Spec, attrReplicas))
	m.Size = types.StringValue(specString(app.Spec, attrSize))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.External = types.BoolValue(specBool(app.Spec, attrExternal))
	m.Version = types.StringValue(specString(app.Spec, attrVersion))
	m.AuthEnabled = types.BoolValue(specBool(app.Spec, specAuthEnabled))
	m.ResourcesPreset = types.StringValue(specString(app.Spec, specResourcesPreset))

	resources, rDiags := flattenResources(app.Spec[attrResources])
	diags.Append(rDiags...)

	m.Resources = resources

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}
