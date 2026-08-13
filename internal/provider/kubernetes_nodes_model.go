package provider

import (
	"context"
	"strings"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// kubernetesNodesModel maps the cozystack_kubernetes_nodes schema to Go types.
// A pool is a standalone worker node group: it carries the same sizing fields a
// nodeGroup of cozystack_kubernetes carries, plus the kubelet, Talos image, and
// image-override blocks that only the standalone kind exposes.
type kubernetesNodesModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Namespace          types.String `tfsdk:"namespace"`
	Cluster            types.String `tfsdk:"cluster"`
	StorageClass       types.String `tfsdk:"storage_class"`
	MinReplicas        types.Int64  `tfsdk:"min_replicas"`
	MaxReplicas        types.Int64  `tfsdk:"max_replicas"`
	InstanceType       types.String `tfsdk:"instance_type"`
	DiskSize           types.String `tfsdk:"disk_size"`
	Roles              types.List   `tfsdk:"roles"`
	Resources          types.Object `tfsdk:"resources"`
	Gpus               types.List   `tfsdk:"gpus"`
	Kubelet            types.Object `tfsdk:"kubelet"`
	MaxUnhealthy       types.String `tfsdk:"max_unhealthy"`
	NodeStartupTimeout types.String `tfsdk:"node_startup_timeout"`
	Version            types.String `tfsdk:"version"`
	Talos              types.Object `tfsdk:"talos"`
	Images             types.Object `tfsdk:"images"`
	Ready              types.Bool   `tfsdk:"ready"`
	ChartVersion       types.String `tfsdk:"chart_version"`
	UID                types.String `tfsdk:"uid"`
}

type kubernetesNodesResourceModel struct {
	kubernetesNodesModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *kubernetesNodesResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

// flatten keeps the chart-owned fields out of the resource's state.
//
// The aggregated API resolves the chart's defaults into the spec on every read,
// so the server answers with a populated `talos`, `kubelet`, and `images` even
// for a pool that set none of them. Storing what it returns would make the next
// update write those values back as explicit spec keys, freezing the pool on the
// Talos release and images that were current the day it was created — while the
// parent cluster, which does not model them at all, keeps following the chart.
// A field the configuration left unset therefore stays null here.
//
// A data source has the opposite job, reporting the values the server resolved,
// so this override is on the resource model alone and the base flatten still
// reports everything the server sent.
//
// The read that follows terraform import is the exception. It is the one call
// that must capture what the pool already has — masking there would hide an
// existing Talos or image override from the plan and let the next update, which
// replaces the spec whole, delete it. That read is identifiable: ImportState
// writes only the identity, so id is still null, while create leaves it unknown
// and every later call has it known.
func (m *kubernetesNodesResourceModel) flatten(app *client.Application) diag.Diagnostics {
	imported := m.ID.IsNull()

	roles, gpus := m.Roles, m.Gpus
	kubelet, talos, images := m.Kubelet, m.Talos, m.Images

	diags := m.kubernetesNodesModel.flatten(app)

	m.Roles = keepConfiguredList(roles, m.Roles, types.StringType)
	m.Gpus = keepConfiguredList(gpus, m.Gpus, types.ObjectType{AttrTypes: vmGpuObjectType()})

	if imported {
		return diags
	}

	m.Kubelet = keepConfiguredFields(kubelet, m.Kubelet, k8sNodesKubeletObjectType(), &diags)
	m.Talos = keepConfiguredFields(talos, m.Talos, k8sNodesTalosObjectType(), &diags)
	m.Images = keepConfiguredFields(images, m.Images, k8sNodesImagesObjectType(), &diags)

	return diags
}

// keepConfiguredList drops a resolved list the configuration did not ask for.
// The chart defaults both lists to empty, so an empty resolved list is the
// server echoing "unset" and is dropped either way; a populated one is real
// state, which is what lets an import capture the roles and GPUs a pool has.
func keepConfiguredList(configured, resolved types.List, elementType attr.Type) types.List {
	if configured.IsNull() && len(resolved.Elements()) == 0 {
		return types.ListNull(elementType)
	}

	return resolved
}

// keepConfiguredFields merges a resolved block over the configured one, keeping
// every field the configuration left null. A block that was not configured at
// all stays null, and a field the server did not answer with keeps what was
// configured, which also covers the server dropping the block entirely.
func keepConfiguredFields(
	configured, resolved types.Object,
	objectType map[string]attr.Type,
	diags *diag.Diagnostics,
) types.Object {
	if configured.IsNull() {
		return types.ObjectNull(objectType)
	}

	resolvedFields := resolved.Attributes()
	fields := make(map[string]attr.Value, len(objectType))

	for name, value := range configured.Attributes() {
		resolvedValue, answered := resolvedFields[name]
		if value.IsNull() || !answered {
			fields[name] = value

			continue
		}

		fields[name] = resolvedValue
	}

	object, objectDiags := types.ObjectValue(objectType, fields)
	diags.Append(objectDiags...)

	return object
}

func (m *kubernetesNodesModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func k8sNodesKubeletObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"eviction_hard_memory":   types.StringType,
		"eviction_soft_memory":   types.StringType,
		"kube_reserved_cpu":      types.StringType,
		"kube_reserved_memory":   types.StringType,
		"system_reserved_cpu":    types.StringType,
		"system_reserved_memory": types.StringType,
	}
}

type k8sNodesKubeletData struct {
	EvictionHardMemory   types.String `tfsdk:"eviction_hard_memory"`
	EvictionSoftMemory   types.String `tfsdk:"eviction_soft_memory"`
	KubeReservedCPU      types.String `tfsdk:"kube_reserved_cpu"`
	KubeReservedMemory   types.String `tfsdk:"kube_reserved_memory"`
	SystemReservedCPU    types.String `tfsdk:"system_reserved_cpu"`
	SystemReservedMemory types.String `tfsdk:"system_reserved_memory"`
}

func k8sNodesTalosObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"version":              types.StringType,
		"schematic_id":         types.StringType,
		"image_factory_url":    types.StringType,
		"installer_repository": types.StringType,
	}
}

type k8sNodesTalosData struct {
	Version             types.String `tfsdk:"version"`
	SchematicID         types.String `tfsdk:"schematic_id"`
	ImageFactoryURL     types.String `tfsdk:"image_factory_url"`
	InstallerRepository types.String `tfsdk:"installer_repository"`
}

func k8sNodesImagesObjectType() map[string]attr.Type {
	return map[string]attr.Type{"kubectl": types.StringType}
}

type k8sNodesImagesData struct {
	Kubectl types.String `tfsdk:"kubectl"`
}

func (m *kubernetesNodesModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	resources, resourceDiags := expandResources(ctx, m.Resources)
	diags.Append(resourceDiags...)

	kubelet, kubeletDiags := expandK8sNodesKubelet(ctx, m.Kubelet)
	diags.Append(kubeletDiags...)

	talos, talosDiags := expandK8sNodesTalos(ctx, m.Talos)
	diags.Append(talosDiags...)

	images, imageDiags := expandK8sNodesImages(ctx, m.Images)
	diags.Append(imageDiags...)

	diags.Append(m.validateConfig()...)

	spec := map[string]any{
		"cluster":            m.Cluster.ValueString(),
		specStorageClass:     m.StorageClass.ValueString(),
		"minReplicas":        m.MinReplicas.ValueInt64(),
		"maxReplicas":        m.MaxReplicas.ValueInt64(),
		"instanceType":       m.InstanceType.ValueString(),
		"diskSize":           m.DiskSize.ValueString(),
		attrResources:        resources,
		"kubelet":            kubelet,
		"maxUnhealthy":       m.MaxUnhealthy.ValueString(),
		"nodeStartupTimeout": m.NodeStartupTimeout.ValueString(),
		attrVersion:          m.Version.ValueString(),
		"talos":              talos,
		"images":             images,
	}

	diags.Append(setOptionalStringList(ctx, spec, "roles", m.Roles)...)

	// A pool's GPU entries have the same {name} shape as a VM instance's — the
	// nodes are KubeVirt VMs — so the VM object type and decode struct are reused.
	diags.Append(setOptionalObjectList(ctx, spec, "gpus", m.Gpus, func(gpu vmNameData) map[string]any {
		return map[string]any{attrName: gpu.Name.ValueString()}
	})...)

	if diags.HasError() {
		return nil, diags
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

// The plan-time guard reaches the pool only through this interface, so the
// binding is asserted here: renaming the method would otherwise disable the
// check silently.
var _ configValidator = (*kubernetesNodesResourceModel)(nil)

// validateConfig enforces the pool's naming rule. The chart derives the pool
// name by stripping the parent cluster from the release name and calls fail when
// the prefix is missing, which lands as a HelmRelease that can never install.
//
// It runs while the plan is built, which is what makes it useful: name and
// cluster both force replacement, so a mistyped rename plans as destroy plus
// create, and a check that waited for Create would fire with the old pool
// already torn down. Values that are still unknown at plan time are left to the
// apply-time call in expand.
func (m *kubernetesNodesModel) validateConfig() diag.Diagnostics {
	var diags diag.Diagnostics

	if m.Name.IsUnknown() || m.Cluster.IsUnknown() {
		return diags
	}

	name, cluster := m.Name.ValueString(), m.Cluster.ValueString()

	prefix := cluster + "-"
	if cluster == "" || (strings.HasPrefix(name, prefix) && len(name) > len(prefix)) {
		return diags
	}

	diags.AddError(
		"Invalid KubernetesNodes name",
		"name must be \""+cluster+"-<pool>\" so the pool attaches to cluster \""+cluster+"\", got \""+name+"\".",
	)

	return diags
}

// expandK8sNodesKubelet renders the kubelet block into a spec submap. Unset
// fields are left out so the chart keeps computing them from the instance type.
func expandK8sNodesKubelet(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if obj.IsNull() || obj.IsUnknown() {
		return out, diags
	}

	var data k8sNodesKubeletData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return out, diags
	}

	setOptionalString(out, "evictionHardMemory", data.EvictionHardMemory)
	setOptionalString(out, "evictionSoftMemory", data.EvictionSoftMemory)
	setOptionalString(out, "kubeReservedCpu", data.KubeReservedCPU)
	setOptionalString(out, "kubeReservedMemory", data.KubeReservedMemory)
	setOptionalString(out, "systemReservedCpu", data.SystemReservedCPU)
	setOptionalString(out, "systemReservedMemory", data.SystemReservedMemory)

	return out, diags
}

// expandK8sNodesTalos renders the talos block into a spec submap. Unset fields
// are left out: the chart's own image factory URL, schematic, and Talos release
// move with the Cozystack release, so pinning them here would freeze a pool on
// whatever those values happened to be when the config was written.
func expandK8sNodesTalos(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if obj.IsNull() || obj.IsUnknown() {
		return out, diags
	}

	var data k8sNodesTalosData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return out, diags
	}

	setOptionalString(out, attrVersion, data.Version)
	setOptionalString(out, "schematicID", data.SchematicID)
	setOptionalString(out, "imageFactoryURL", data.ImageFactoryURL)
	setOptionalString(out, "installerRepository", data.InstallerRepository)

	return out, diags
}

// expandK8sNodesImages renders the images block into a spec submap, leaving an
// unset override out so the chart's pinned image tag applies.
func expandK8sNodesImages(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if obj.IsNull() || obj.IsUnknown() {
		return out, diags
	}

	var data k8sNodesImagesData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return out, diags
	}

	setOptionalString(out, "kubectl", data.Kubectl)

	return out, diags
}

func (m *kubernetesNodesModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Cluster = types.StringValue(specString(app.Spec, "cluster"))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.MinReplicas = types.Int64Value(specInt64(app.Spec, "minReplicas"))
	m.MaxReplicas = types.Int64Value(specInt64(app.Spec, "maxReplicas"))
	m.InstanceType = types.StringValue(specString(app.Spec, "instanceType"))
	m.DiskSize = types.StringValue(specString(app.Spec, "diskSize"))
	m.Roles = specStringListOrNull(app.Spec["roles"])
	m.MaxUnhealthy = types.StringValue(specString(app.Spec, "maxUnhealthy"))
	m.NodeStartupTimeout = types.StringValue(specString(app.Spec, "nodeStartupTimeout"))
	m.Version = types.StringValue(specString(app.Spec, attrVersion))

	resources, resourceDiags := flattenResources(app.Spec[attrResources])
	diags.Append(resourceDiags...)

	m.Resources = resources

	gpus, gpuDiags := specObjectListOrNull(app.Spec["gpus"], vmGpuObjectType(), func(gpu map[string]any) map[string]attr.Value {
		return map[string]attr.Value{attrName: types.StringValue(specString(gpu, attrName))}
	})
	diags.Append(gpuDiags...)

	m.Gpus = gpus

	diags.Append(m.flattenBlocks(app.Spec)...)

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}

// flattenBlocks reads the kubelet, talos, and images sub-objects. Each field is
// read back presence-preserving: a key the server never stored flattens to null,
// while a key stored empty keeps its empty value, so an operator who cleared a
// field does not see it reappear as drift on the next plan.
func (m *kubernetesNodesModel) flattenBlocks(spec map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics

	kubelet, kubeletDiags := flattenSpecObject(spec["kubelet"], k8sNodesKubeletObjectType(),
		func(fields map[string]any) map[string]attr.Value {
			return map[string]attr.Value{
				"eviction_hard_memory":   specStringOrNull(fields, "evictionHardMemory"),
				"eviction_soft_memory":   specStringOrNull(fields, "evictionSoftMemory"),
				"kube_reserved_cpu":      specStringOrNull(fields, "kubeReservedCpu"),
				"kube_reserved_memory":   specStringOrNull(fields, "kubeReservedMemory"),
				"system_reserved_cpu":    specStringOrNull(fields, "systemReservedCpu"),
				"system_reserved_memory": specStringOrNull(fields, "systemReservedMemory"),
			}
		})
	diags.Append(kubeletDiags...)

	m.Kubelet = kubelet

	talos, talosDiags := flattenSpecObject(spec["talos"], k8sNodesTalosObjectType(),
		func(fields map[string]any) map[string]attr.Value {
			return map[string]attr.Value{
				"version":              specStringOrNull(fields, attrVersion),
				"schematic_id":         specStringOrNull(fields, "schematicID"),
				"image_factory_url":    specStringOrNull(fields, "imageFactoryURL"),
				"installer_repository": specStringOrNull(fields, "installerRepository"),
			}
		})
	diags.Append(talosDiags...)

	m.Talos = talos

	images, imageDiags := flattenSpecObject(spec["images"], k8sNodesImagesObjectType(),
		func(fields map[string]any) map[string]attr.Value {
			return map[string]attr.Value{"kubectl": specStringOrNull(fields, "kubectl")}
		})
	diags.Append(imageDiags...)

	m.Images = images

	return diags
}

// flattenSpecObject builds a single nested object from a spec submap,
// delegating field conversion to build. An absent or empty submap flattens to
// null so an unset block does not drift.
func flattenSpecObject(
	raw any,
	objectType map[string]attr.Type,
	build func(map[string]any) map[string]attr.Value,
) (types.Object, diag.Diagnostics) {
	fields, ok := raw.(map[string]any)
	if !ok || len(fields) == 0 {
		return types.ObjectNull(objectType), nil
	}

	return types.ObjectValue(objectType, build(fields))
}
