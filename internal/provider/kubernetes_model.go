package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// kubernetesModel maps the cozystack_kubernetes schema to Go types. The addons,
// controlPlane, and images blocks, as well as per-node-group GPU and kubelet
// tuning, are not managed (they use server defaults).
type kubernetesModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	StorageClass types.String `tfsdk:"storage_class"`
	Version      types.String `tfsdk:"version"`
	Host         types.String `tfsdk:"host"`
	NodeGroups   types.Map    `tfsdk:"node_groups"`
	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
	Kubeconfig   types.String `tfsdk:"kubeconfig"`
}

type kubernetesResourceModel struct {
	kubernetesModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *kubernetesResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *kubernetesModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func k8sNodeGroupObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"disk_size":     types.StringType,
		"instance_type": types.StringType,
		"min_replicas":  types.Int64Type,
		"max_replicas":  types.Int64Type,
		"roles":         types.ListType{ElemType: types.StringType},
		"storage_class": types.StringType,
		"resources":     types.ObjectType{AttrTypes: resourcesObjectType()},
	}
}

type k8sNodeGroupData struct {
	DiskSize     types.String `tfsdk:"disk_size"`
	InstanceType types.String `tfsdk:"instance_type"`
	MinReplicas  types.Int64  `tfsdk:"min_replicas"`
	MaxReplicas  types.Int64  `tfsdk:"max_replicas"`
	Roles        []string     `tfsdk:"roles"`
	StorageClass types.String `tfsdk:"storage_class"`
	Resources    types.Object `tfsdk:"resources"`
}

func (m *kubernetesModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	nodeGroups, ngDiags := expandNodeGroups(ctx, m.NodeGroups)
	diags.Append(ngDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		specStorageClass: m.StorageClass.ValueString(),
		attrVersion:      m.Version.ValueString(),
		"nodeGroups":     nodeGroups,
	}

	// host is server-defaulted to a tenant subdomain; only send it when set so
	// the computed default does not produce a perpetual diff.
	if host := m.Host.ValueString(); host != "" {
		spec[attrHost] = host
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func expandNodeGroups(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]k8sNodeGroupData{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name := range elements {
		group := elements[name]

		resources, rDiags := expandResources(ctx, group.Resources)
		diags.Append(rDiags...)

		entry := map[string]any{
			"diskSize":       group.DiskSize.ValueString(),
			"instanceType":   group.InstanceType.ValueString(),
			"minReplicas":    group.MinReplicas.ValueInt64(),
			"maxReplicas":    group.MaxReplicas.ValueInt64(),
			specStorageClass: group.StorageClass.ValueString(),
			attrResources:    resources,
		}

		if len(group.Roles) > 0 {
			entry["roles"] = stringsToAny(group.Roles)
		}

		out[name] = entry
	}

	return out, diags
}

func (m *kubernetesModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.Version = types.StringValue(specString(app.Spec, attrVersion))
	m.Host = types.StringValue(specString(app.Spec, attrHost))

	nodeGroups, ngDiags := flattenNodeGroups(app.Spec["nodeGroups"])
	diags.Append(ngDiags...)

	m.NodeGroups = nodeGroups

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}

// readOutputs reads the cluster's admin kubeconfig, which the chart materialises
// as the Secret `kubernetes-<name>-admin-kubeconfig` (key `super-admin.conf`). It
// is created asynchronously, so an absent Secret leaves the attribute null.
func (m *kubernetesModel) readOutputs(ctx context.Context, api *client.Client) diag.Diagnostics {
	var diags diag.Diagnostics

	m.Kubeconfig = types.StringNull()

	namespace, name := m.identity()

	data, found, err := api.GetSecretData(ctx, namespace, "kubernetes-"+name+"-admin-kubeconfig")
	if err != nil {
		diags.AddError("Unable to read Kubernetes admin kubeconfig", err.Error())

		return diags
	}

	if found {
		if conf, ok := data["super-admin.conf"]; ok {
			m.Kubeconfig = types.StringValue(string(conf))
		}
	}

	return diags
}

func flattenNodeGroups(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, k8sNodeGroupObjectType(), func(group map[string]any) map[string]attr.Value {
		resources, _ := flattenResources(group[attrResources])

		return map[string]attr.Value{
			"disk_size":     types.StringValue(specString(group, "diskSize")),
			"instance_type": types.StringValue(specString(group, "instanceType")),
			"min_replicas":  types.Int64Value(anyToInt64(group["minReplicas"])),
			"max_replicas":  types.Int64Value(anyToInt64(group["maxReplicas"])),
			"roles":         stringListOrNull(group["roles"]),
			"storage_class": types.StringValue(specString(group, specStorageClass)),
			"resources":     resources,
		}
	})
}
