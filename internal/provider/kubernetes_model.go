package provider

import (
	"context"
	"encoding/json"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Spec keys of the cozystack_kubernetes blocks that carry no shared constant.
const (
	specNodeGroups         = "nodeGroups"
	specTalos              = "talos"
	specNodeHealthCheck    = "nodeHealthCheck"
	specMaxUnhealthy       = "maxUnhealthy"
	specNodeStartupTimeout = "nodeStartupTimeout"
	specOIDC               = "oidc"
	specCustomConfig       = "customConfig"
	specSecretRef          = "secretRef"
	specControlPlane       = "controlPlane"
	specAPIServer          = "apiServer"
)

// kubernetesModel maps the cozystack_kubernetes schema to Go types. The addons
// block, the control-plane component sizing, and per-node-group GPU and kubelet
// tuning are not managed (they use server defaults).
//
// Blocks whose upstream defaults roll with each Cozystack release (talos) are
// modelled without provider-side defaults: an unset block is left out of the
// emitted spec so the aggregated apiserver keeps supplying its own current
// value, which it materialises on every read.
type kubernetesModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	StorageClass types.String `tfsdk:"storage_class"`
	Version      types.String `tfsdk:"version"`
	Host         types.String `tfsdk:"host"`
	NodeGroups   types.Map    `tfsdk:"node_groups"`

	Talos           types.Object `tfsdk:"talos"`
	NodeHealthCheck types.Object `tfsdk:"node_health_check"`
	OIDC            types.Object `tfsdk:"oidc"`
	ControlPlane    types.Object `tfsdk:"control_plane"`

	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
	UID          types.String `tfsdk:"uid"`
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
		"disk_size":            types.StringType,
		"instance_type":        types.StringType,
		"min_replicas":         types.Int64Type,
		"max_replicas":         types.Int64Type,
		"roles":                types.ListType{ElemType: types.StringType},
		"storage_class":        types.StringType,
		"resources":            types.ObjectType{AttrTypes: resourcesObjectType()},
		"max_unhealthy":        types.StringType,
		"node_startup_timeout": types.StringType,
	}
}

type k8sNodeGroupData struct {
	DiskSize           types.String `tfsdk:"disk_size"`
	InstanceType       types.String `tfsdk:"instance_type"`
	MinReplicas        types.Int64  `tfsdk:"min_replicas"`
	MaxReplicas        types.Int64  `tfsdk:"max_replicas"`
	Roles              []string     `tfsdk:"roles"`
	StorageClass       types.String `tfsdk:"storage_class"`
	Resources          types.Object `tfsdk:"resources"`
	MaxUnhealthy       types.String `tfsdk:"max_unhealthy"`
	NodeStartupTimeout types.String `tfsdk:"node_startup_timeout"`
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
		specNodeGroups:   nodeGroups,
	}

	// host is server-defaulted to a tenant subdomain; only send it when set so
	// the computed default does not produce a perpetual diff.
	if host := m.Host.ValueString(); host != "" {
		spec[attrHost] = host
	}

	diags.Append(m.expandBlocks(ctx, spec)...)

	if diags.HasError() {
		return nil, diags
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

		// Both overrides are undefaulted upstream: an absent key means the
		// cluster-wide nodeHealthCheck applies to this group.
		setOptionalString(entry, specMaxUnhealthy, group.MaxUnhealthy)
		setOptionalString(entry, specNodeStartupTimeout, group.NodeStartupTimeout)

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

	nodeGroups, ngDiags := flattenNodeGroups(app.Spec[specNodeGroups])
	diags.Append(ngDiags...)

	m.NodeGroups = nodeGroups
	m.Talos = flattenTalos(app.Spec[specTalos])
	m.NodeHealthCheck = flattenNodeHealthCheck(app.Spec[specNodeHealthCheck])

	oidc, oidcDiags := flattenOIDC(app.Spec[specOIDC])
	diags.Append(oidcDiags...)

	m.OIDC = oidc

	controlPlane, cpDiags := flattenControlPlane(app.Spec[specControlPlane])
	diags.Append(cpDiags...)

	m.ControlPlane = controlPlane

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

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

// expandBlocks renders the optional nested blocks into spec. Each block is
// written only when the practitioner set it, so an unset block leaves the key
// out and the server's own default stands.
func (m *kubernetesModel) expandBlocks(ctx context.Context, spec map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics

	blocks := []struct {
		key    string
		value  types.Object
		expand func(context.Context, types.Object) (map[string]any, diag.Diagnostics)
	}{
		{key: specTalos, value: m.Talos, expand: expandTalos},
		{key: specNodeHealthCheck, value: m.NodeHealthCheck, expand: expandNodeHealthCheck},
		{key: specOIDC, value: m.OIDC, expand: expandOIDC},
		{key: specControlPlane, value: m.ControlPlane, expand: expandControlPlane},
	}

	for _, block := range blocks {
		out, blockDiags := block.expand(ctx, block.value)
		diags.Append(blockDiags...)

		if out != nil {
			spec[block.key] = out
		}
	}

	return diags
}

func k8sTalosObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"image_factory_url":    types.StringType,
		"installer_repository": types.StringType,
		"schematic_id":         types.StringType,
		attrVersion:            types.StringType,
	}
}

type k8sTalosData struct {
	ImageFactoryURL     types.String `tfsdk:"image_factory_url"`
	InstallerRepository types.String `tfsdk:"installer_repository"`
	SchematicID         types.String `tfsdk:"schematic_id"`
	Version             types.String `tfsdk:"version"`
}

// expandTalos renders the talos block into a spec submap, or nil when the block
// is unset. Each field is emitted only when set: the upstream defaults name a
// specific Talos release and image-factory schematic, both of which move with
// every Cozystack release, so writing the provider's idea of them would pin the
// cluster to a stale image.
func expandTalos(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var data k8sTalosData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{}

	setOptionalString(out, "imageFactoryURL", data.ImageFactoryURL)
	setOptionalString(out, "installerRepository", data.InstallerRepository)
	setOptionalString(out, "schematicID", data.SchematicID)
	setOptionalString(out, attrVersion, data.Version)

	return out, diags
}

// flattenTalos builds the talos block from a spec submap. An absent key
// flattens to null; the aggregated apiserver materialises the block's schema
// defaults on read, so a managed cluster normally reports every field.
func flattenTalos(raw any) types.Object {
	talos, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(k8sTalosObjectType())
	}

	return types.ObjectValueMust(k8sTalosObjectType(), map[string]attr.Value{
		"image_factory_url":    specStringOrNull(talos, "imageFactoryURL"),
		"installer_repository": specStringOrNull(talos, "installerRepository"),
		"schematic_id":         specStringOrNull(talos, "schematicID"),
		attrVersion:            specStringOrNull(talos, attrVersion),
	})
}

func k8sNodeHealthCheckObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"max_unhealthy":        types.StringType,
		"node_startup_timeout": types.StringType,
	}
}

type k8sNodeHealthCheckData struct {
	MaxUnhealthy       types.String `tfsdk:"max_unhealthy"`
	NodeStartupTimeout types.String `tfsdk:"node_startup_timeout"`
}

// expandNodeHealthCheck renders the nodeHealthCheck block into a spec submap,
// or nil when the block is unset.
func expandNodeHealthCheck(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var data k8sNodeHealthCheckData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{}

	setOptionalString(out, specMaxUnhealthy, data.MaxUnhealthy)
	setOptionalString(out, specNodeStartupTimeout, data.NodeStartupTimeout)

	return out, diags
}

// flattenNodeHealthCheck builds the nodeHealthCheck block from a spec submap.
func flattenNodeHealthCheck(raw any) types.Object {
	check, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(k8sNodeHealthCheckObjectType())
	}

	return types.ObjectValueMust(k8sNodeHealthCheckObjectType(), map[string]attr.Value{
		"max_unhealthy":        specStringOrNull(check, specMaxUnhealthy),
		"node_startup_timeout": specStringOrNull(check, specNodeStartupTimeout),
	})
}

func k8sOIDCObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"mode":          types.StringType,
		"users":         types.ListType{ElemType: types.ObjectType{AttrTypes: k8sOIDCUserObjectType()}},
		"custom_config": types.ObjectType{AttrTypes: k8sOIDCCustomConfigObjectType()},
	}
}

func k8sOIDCUserObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"email": types.StringType,
		"role":  types.StringType,
	}
}

func k8sOIDCCustomConfigObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"config":     types.StringType,
		"secret_ref": types.ObjectType{AttrTypes: k8sOIDCSecretRefObjectType()},
	}
}

func k8sOIDCSecretRefObjectType() map[string]attr.Type {
	return map[string]attr.Type{attrName: types.StringType}
}

type k8sOIDCData struct {
	Mode         types.String `tfsdk:"mode"`
	Users        types.List   `tfsdk:"users"`
	CustomConfig types.Object `tfsdk:"custom_config"`
}

type k8sOIDCUserData struct {
	Email types.String `tfsdk:"email"`
	Role  types.String `tfsdk:"role"`
}

type k8sOIDCCustomConfigData struct {
	Config    types.String `tfsdk:"config"`
	SecretRef types.Object `tfsdk:"secret_ref"`
}

type k8sOIDCSecretRefData struct {
	Name types.String `tfsdk:"name"`
}

// expandOIDC renders the oidc block into a spec submap, or nil when the block
// is unset. users keeps its presence distinction: the platform defaults the key
// to an empty list, so an explicitly empty list is an operator saying "bind no
// users" and must reach the server as written.
func expandOIDC(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var data k8sOIDCData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{}

	setOptionalString(out, "mode", data.Mode)
	diags.Append(setOptionalObjectList(ctx, out, "users", data.Users, buildOIDCUser)...)

	custom, customDiags := expandOIDCCustomConfig(ctx, data.CustomConfig)
	diags.Append(customDiags...)

	if custom != nil {
		out[specCustomConfig] = custom
	}

	return out, diags
}

func buildOIDCUser(user k8sOIDCUserData) map[string]any {
	return map[string]any{
		"email": user.Email.ValueString(),
		"role":  user.Role.ValueString(),
	}
}

// expandOIDCCustomConfig renders the oidc.customConfig block, or nil when it is
// unset.
func expandOIDCCustomConfig(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var data k8sOIDCCustomConfigData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{}

	setOptionalString(out, "config", data.Config)

	if data.SecretRef.IsNull() || data.SecretRef.IsUnknown() {
		return out, diags
	}

	var ref k8sOIDCSecretRefData

	diags.Append(data.SecretRef.As(ctx, &ref, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	secretRef := map[string]any{}
	setOptionalString(secretRef, attrName, ref.Name)
	out[specSecretRef] = secretRef

	return out, diags
}

// flattenOIDC builds the oidc block from a spec submap. An absent key flattens
// to null; every other value is read presence-preserving, since the platform
// materialises this block's defaults on read and an empty string there is a
// real value rather than an unset field.
func flattenOIDC(raw any) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	oidc, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(k8sOIDCObjectType()), diags
	}

	users, userDiags := specObjectListOrNull(oidc["users"], k8sOIDCUserObjectType(), flattenOIDCUser)
	diags.Append(userDiags...)

	custom, customDiags := flattenOIDCCustomConfig(oidc[specCustomConfig])
	diags.Append(customDiags...)

	value, valueDiags := types.ObjectValue(k8sOIDCObjectType(), map[string]attr.Value{
		"mode":          specStringOrNull(oidc, "mode"),
		"users":         users,
		"custom_config": custom,
	})
	diags.Append(valueDiags...)

	return value, diags
}

func flattenOIDCUser(user map[string]any) map[string]attr.Value {
	return map[string]attr.Value{
		"email": types.StringValue(specString(user, "email")),
		"role":  types.StringValue(specString(user, "role")),
	}
}

func flattenOIDCCustomConfig(raw any) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	custom, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(k8sOIDCCustomConfigObjectType()), diags
	}

	secretRef := types.ObjectNull(k8sOIDCSecretRefObjectType())

	if ref, ok := custom[specSecretRef].(map[string]any); ok {
		value, refDiags := types.ObjectValue(k8sOIDCSecretRefObjectType(), map[string]attr.Value{
			attrName: specStringOrNull(ref, attrName),
		})
		diags.Append(refDiags...)

		secretRef = value
	}

	value, valueDiags := types.ObjectValue(k8sOIDCCustomConfigObjectType(), map[string]attr.Value{
		"config":     specStringOrNull(custom, "config"),
		"secret_ref": secretRef,
	})
	diags.Append(valueDiags...)

	return value, diags
}

func k8sControlPlaneObjectType() map[string]attr.Type {
	return map[string]attr.Type{"api_server": types.ObjectType{AttrTypes: k8sAPIServerObjectType()}}
}

func k8sAPIServerObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"extra_args":          types.ListType{ElemType: types.StringType},
		"extra_volumes":       types.ListType{ElemType: jsontypes.NormalizedType{}},
		"extra_volume_mounts": types.ListType{ElemType: jsontypes.NormalizedType{}},
	}
}

type k8sControlPlaneData struct {
	APIServer types.Object `tfsdk:"api_server"`
}

type k8sAPIServerData struct {
	ExtraArgs         types.List `tfsdk:"extra_args"`
	ExtraVolumes      types.List `tfsdk:"extra_volumes"`
	ExtraVolumeMounts types.List `tfsdk:"extra_volume_mounts"`
}

// expandControlPlane renders the controlPlane block, or nil when it is unset.
// Only the apiServer passthrough is modelled; component sizing and the replica
// count stay with the server.
func expandControlPlane(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var data k8sControlPlaneData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{}

	apiServer, apiDiags := expandAPIServer(ctx, data.APIServer)
	diags.Append(apiDiags...)

	if apiServer != nil {
		out[specAPIServer] = apiServer
	}

	return out, diags
}

func expandAPIServer(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var data k8sAPIServerData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{}

	diags.Append(setOptionalStringList(ctx, out, "extraArgs", data.ExtraArgs)...)
	diags.Append(setOptionalJSONList(ctx, out, "extraVolumes", data.ExtraVolumes)...)
	diags.Append(setOptionalJSONList(ctx, out, "extraVolumeMounts", data.ExtraVolumeMounts)...)

	return out, diags
}

// setOptionalJSONList writes a list of JSON documents into spec under key. The
// upstream fields are free-form core/v1 objects, so they travel as normalized
// JSON strings rather than a hand-modelled Volume schema.
func setOptionalJSONList(
	ctx context.Context,
	spec map[string]any,
	key string,
	value types.List,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if value.IsNull() || value.IsUnknown() {
		return diags
	}

	var items []jsontypes.Normalized

	diags.Append(value.ElementsAs(ctx, &items, false)...)

	if diags.HasError() {
		return diags
	}

	out := make([]any, 0, len(items))

	for _, item := range items {
		var document any

		if err := json.Unmarshal([]byte(item.ValueString()), &document); err != nil {
			diags.AddError("Invalid JSON document in "+key, err.Error())

			return diags
		}

		out = append(out, document)
	}

	spec[key] = out

	return diags
}

// specJSONListOrNull builds a list of JSON documents from a spec value. An
// absent key flattens to null; a present empty list stays an empty list.
func specJSONListOrNull(raw any) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	items, ok := raw.([]any)
	if !ok {
		return types.ListNull(jsontypes.NormalizedType{}), diags
	}

	elements := make([]attr.Value, 0, len(items))

	for _, item := range items {
		encoded, err := json.Marshal(item)
		if err != nil {
			diags.AddError("Unable to encode a spec document", err.Error())

			return types.ListNull(jsontypes.NormalizedType{}), diags
		}

		elements = append(elements, jsontypes.NewNormalizedValue(string(encoded)))
	}

	value, listDiags := types.ListValue(jsontypes.NormalizedType{}, elements)
	diags.Append(listDiags...)

	return value, diags
}

// flattenControlPlane builds the controlPlane block from a spec submap.
func flattenControlPlane(raw any) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	controlPlane, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(k8sControlPlaneObjectType()), diags
	}

	apiServer, apiDiags := flattenAPIServer(controlPlane[specAPIServer])
	diags.Append(apiDiags...)

	value, valueDiags := types.ObjectValue(k8sControlPlaneObjectType(), map[string]attr.Value{
		"api_server": apiServer,
	})
	diags.Append(valueDiags...)

	return value, diags
}

func flattenAPIServer(raw any) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	apiServer, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(k8sAPIServerObjectType()), diags
	}

	volumes, volumeDiags := specJSONListOrNull(apiServer["extraVolumes"])
	diags.Append(volumeDiags...)

	mounts, mountDiags := specJSONListOrNull(apiServer["extraVolumeMounts"])
	diags.Append(mountDiags...)

	value, valueDiags := types.ObjectValue(k8sAPIServerObjectType(), map[string]attr.Value{
		"extra_args":          specStringListOrNull(apiServer["extraArgs"]),
		"extra_volumes":       volumes,
		"extra_volume_mounts": mounts,
	})
	diags.Append(valueDiags...)

	return value, diags
}

func flattenNodeGroups(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, k8sNodeGroupObjectType(), func(group map[string]any) map[string]attr.Value {
		resources, _ := flattenResources(group[attrResources])

		return map[string]attr.Value{
			"disk_size":            types.StringValue(specString(group, "diskSize")),
			"instance_type":        types.StringValue(specString(group, "instanceType")),
			"min_replicas":         types.Int64Value(anyToInt64(group["minReplicas"])),
			"max_replicas":         types.Int64Value(anyToInt64(group["maxReplicas"])),
			"roles":                stringListOrNull(group["roles"]),
			"storage_class":        types.StringValue(specString(group, specStorageClass)),
			"resources":            resources,
			"max_unhealthy":        specStringOrNull(group, specMaxUnhealthy),
			"node_startup_timeout": specStringOrNull(group, specNodeStartupTimeout),
		}
	})
}
