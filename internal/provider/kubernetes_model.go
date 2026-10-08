package provider

import (
	"context"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Blocks whose Terraform attribute name and spec key are the same word, so one
// constant serves the schema, expand, and flatten alike.
const (
	attrTalos  = "talos"
	attrOIDC   = "oidc"
	attrImages = "images"
)

// Attribute name of the block whose spec key is the camelCase form.
const attrControlPlane = "control_plane"

// Spec keys of the cozystack_kubernetes blocks whose camelCase form differs
// from the snake_case attribute name.
const (
	specCustomConfig = "customConfig"
	specSecretRef    = "secretRef"
	specControlPlane = "controlPlane"
	specAPIServer    = "apiServer"
)

// kubernetesModel maps the cozystack_kubernetes schema to Go types. The addons
// block and the control-plane component sizing are not managed (they use server
// defaults). Worker pools are not part of this Kind: they are
// cozystack_kubernetes_nodes resources.
//
// Blocks whose upstream defaults roll with each Cozystack release (talos,
// images) are modelled without provider-side defaults: an unset block is left
// out of the emitted spec so the aggregated apiserver keeps supplying its own
// current value. This model reports what the server returns, which is what a
// data source wants; the managed resource narrows that to the configured fields
// in its own flatten.
type kubernetesModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	StorageClass types.String `tfsdk:"storage_class"`
	Version      types.String `tfsdk:"version"`
	Host         types.String `tfsdk:"host"`

	Talos        types.Object `tfsdk:"talos"`
	OIDC         types.Object `tfsdk:"oidc"`
	ControlPlane types.Object `tfsdk:"control_plane"`
	Images       types.Object `tfsdk:"images"`

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

// flatten keeps the managed resource's view of the platform blocks limited to
// what the configuration asked for.
//
// The aggregated apiserver materialises this Kind's schema defaults on every
// read, so the plain model flatten — the one the data source wants — reports the
// whole talos, oidc, controlPlane and images surface whether or
// not anybody configured it. Letting that into a managed resource's state makes
// Terraform's own rules work against the practitioner: the value becomes the
// prior state, the prior state becomes the plan wherever the configuration is
// silent, and the next update writes the platform's defaults into the release as
// though they had been chosen — freezing the cluster on the Talos release and
// schematic that happened to be current that day.
//
// Trimming the blocks back to their configured shape keeps an unwritten field
// out of the request for the life of the resource. A field that was configured
// still refreshes, so drift against it is still visible; a field that was not
// stays null, and the platform stays in charge of it. The data source is the
// place to read the effective values.
func (m *kubernetesResourceModel) flatten(app *client.Application) diag.Diagnostics {
	ctx := context.Background()

	// Snapshot the configured values before the embedded flatten overwrites
	// them from the server response — that ordering is the whole mechanism. One
	// slice of pairs rather than two parallel ones, so a block added to the
	// targets and forgotten in the configured values cannot silently trim
	// against its neighbour.
	blocks := []struct {
		configured types.Object
		target     *types.Object
	}{
		{configured: m.Talos, target: &m.Talos},
		{configured: m.OIDC, target: &m.OIDC},
		{configured: m.ControlPlane, target: &m.ControlPlane},
		{configured: m.Images, target: &m.Images},
	}

	diags := m.kubernetesModel.flatten(app)

	for _, block := range blocks {
		trimmed, trimDiags := keepConfiguredAttributes(ctx, block.configured, *block.target)
		diags.Append(trimDiags...)

		*block.target = trimmed
	}

	return diags
}

// keepConfiguredAttributes returns the server's view of a block with every
// attribute the configuration left unset reset to null, so state never gains a
// value the practitioner did not ask for. A block that was not configured at all
// stays absent. Nested objects are trimmed the same way. Every other configured
// attribute — a list included — takes the server's value, which is what makes
// drift against something the configuration does name visible.
func keepConfiguredAttributes(ctx context.Context, configured, server types.Object) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	if configured.IsNull() || configured.IsUnknown() {
		return types.ObjectNull(server.AttributeTypes(ctx)), diags
	}

	kept := make(map[string]attr.Value, len(configured.Attributes()))

	for name, value := range configured.Attributes() {
		// A server that does not report the key at all — an older cluster, or a
		// block it does not default — leaves the attribute null rather than
		// carrying a configured value that may still be unknown.
		fromServer, ok := server.Attributes()[name]
		if !ok {
			null, nullDiags := nullOf(ctx, value)
			diags.Append(nullDiags...)

			kept[name] = null

			continue
		}

		if nested, isObject := value.(types.Object); isObject {
			serverNested, sameShape := fromServer.(types.Object)
			if !sameShape {
				// The two sides are built from one AttrTypes, so this cannot
				// happen; keeping the configured value beats recursing into a
				// value whose attribute types are unknown.
				kept[name] = nested

				continue
			}

			trimmed, nestedDiags := keepConfiguredAttributes(ctx, nested, serverNested)
			diags.Append(nestedDiags...)

			kept[name] = trimmed

			continue
		}

		if value.IsNull() {
			kept[name] = value

			continue
		}

		kept[name] = fromServer
	}

	value, valueDiags := types.ObjectValue(configured.AttributeTypes(ctx), kept)
	diags.Append(valueDiags...)

	return value, diags
}

// nullOf returns the null of value's own type, for an attribute the server did
// not report back. It works off the value's type rather than a switch over the
// kinds these blocks happen to use today, so adding a bool or a number to one of
// them cannot quietly produce a value of the wrong type.
func nullOf(ctx context.Context, value attr.Value) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	valueType := value.Type(ctx)

	null, err := valueType.ValueFromTerraform(ctx, tftypes.NewValue(valueType.TerraformType(ctx), nil))
	if err != nil {
		diags.AddError("Unable to build a null value", err.Error())

		return value, diags
	}

	return null, diags
}

func (m *kubernetesModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func (m *kubernetesModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	spec := map[string]any{
		attrVersion: m.Version.ValueString(),
	}

	// storageClass carries no provider-side default, so a create that does not
	// name it leaves the key out and the platform supplies "replicated" itself.
	// On later applies the plan holds the value the last read reported, so the
	// key is written back — a no-op by construction, and the point is what does
	// not happen: a materialised default would instead pull the plan back to
	// "replicated" for a cluster created on another class, silently rewriting a
	// field whose PersistentVolumeClaims can never follow.
	setOptionalString(spec, specStorageClass, m.StorageClass)

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

func (m *kubernetesModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.Version = types.StringValue(specString(app.Spec, attrVersion))
	m.Host = types.StringValue(specString(app.Spec, attrHost))

	m.Talos = flattenTalos(app.Spec[attrTalos])

	oidc, oidcDiags := flattenOIDC(app.Spec[attrOIDC])
	diags.Append(oidcDiags...)

	m.OIDC = oidc

	controlPlane, cpDiags := flattenControlPlane(app.Spec[specControlPlane])
	diags.Append(cpDiags...)

	m.ControlPlane = controlPlane
	m.Images = flattenImages(app.Spec[attrImages])

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

// outputsPending reports whether the admin kubeconfig Secret is still absent.
func (m *kubernetesModel) outputsPending() bool {
	return m.Kubeconfig.IsNull()
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
		{key: attrTalos, value: m.Talos, expand: expandTalos},
		{key: attrOIDC, value: m.OIDC, expand: expandOIDC},
		{key: specControlPlane, value: m.ControlPlane, expand: expandControlPlane},
		{key: attrImages, value: m.Images, expand: expandImages},
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
// is unset. An explicitly empty users list is written as written — the chart
// ranges over the list and defaults it to empty, so it lands the same as an
// absent key on the cluster, but sending what the configuration says keeps the
// request a faithful copy of it.
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

	if len(secretRef) > 0 {
		out[specSecretRef] = secretRef
	}

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

func k8sImagesObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"kubectl":             types.StringType,
		"talos_csr_signer":    types.StringType,
		"wait_for_kubeconfig": types.StringType,
	}
}

type k8sImagesData struct {
	Kubectl           types.String `tfsdk:"kubectl"`
	TalosCsrSigner    types.String `tfsdk:"talos_csr_signer"`
	WaitForKubeconfig types.String `tfsdk:"wait_for_kubeconfig"`
}

// expandImages renders the images block into a spec submap, or nil when the
// block is unset. The overrides carry no provider-side default: upstream ships
// a pinned tag per release and an empty value selects it, so pinning one here
// would hold a cluster on an image the chart has already moved past.
func expandImages(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var data k8sImagesData

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{}

	setOptionalString(out, "kubectl", data.Kubectl)
	setOptionalString(out, "talosCsrSigner", data.TalosCsrSigner)
	setOptionalString(out, "waitForKubeconfig", data.WaitForKubeconfig)

	return out, diags
}

// flattenImages builds the images block from a spec submap.
func flattenImages(raw any) types.Object {
	images, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(k8sImagesObjectType())
	}

	return types.ObjectValueMust(k8sImagesObjectType(), map[string]attr.Value{
		"kubectl":             specStringOrNull(images, "kubectl"),
		"talos_csr_signer":    specStringOrNull(images, "talosCsrSigner"),
		"wait_for_kubeconfig": specStringOrNull(images, "waitForKubeconfig"),
	})
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

	apiServer, apiDiags := expandAPIServer(ctx, data.APIServer)
	diags.Append(apiDiags...)

	if apiServer == nil {
		return nil, diags
	}

	return map[string]any{specAPIServer: apiServer}, diags
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
