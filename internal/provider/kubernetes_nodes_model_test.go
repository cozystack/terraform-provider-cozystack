package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/kubernetesnodes"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func fullKubernetesNodesModel() kubernetesNodesModel {
	gpu := types.ObjectValueMust(vmGpuObjectType(), map[string]attr.Value{
		attrName: types.StringValue("nvidia.com/AD102GL_L40S"),
	})

	return kubernetesNodesModel{
		Name:         types.StringValue("demo-md1"),
		Namespace:    types.StringValue("tenant-root"),
		Cluster:      types.StringValue("demo"),
		StorageClass: types.StringValue("replicated"),
		MinReplicas:  types.Int64Value(1),
		MaxReplicas:  types.Int64Value(5),
		InstanceType: types.StringValue("u1.medium"),
		DiskSize:     types.StringValue("20Gi"),
		Roles:        types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ingress-nginx")}),
		Resources: types.ObjectValueMust(resourcesObjectType(), map[string]attr.Value{
			attrCPU:    types.StringValue("4"),
			attrMemory: types.StringValue("8Gi"),
		}),
		Gpus:               types.ListValueMust(types.ObjectType{AttrTypes: vmGpuObjectType()}, []attr.Value{gpu}),
		Kubelet:            fullKubernetesNodesKubelet(),
		MaxUnhealthy:       types.StringValue("50%"),
		NodeStartupTimeout: types.StringValue("10m"),
		Version:            types.StringValue("v1.35"),
		Talos: types.ObjectValueMust(k8sNodesTalosObjectType(), map[string]attr.Value{
			"version":              types.StringValue("v1.13.6"),
			"schematic_id":         types.StringValue("ce4c9805"),
			"image_factory_url":    types.StringValue("https://factory.example.com"),
			"installer_repository": types.StringValue("registry.example.com/installer"),
		}),
		Images: types.ObjectValueMust(k8sNodesImagesObjectType(), map[string]attr.Value{
			"kubectl": types.StringValue("registry.example.com/kubectl:v1.35.0"),
		}),
	}
}

func fullKubernetesNodesKubelet() types.Object {
	return types.ObjectValueMust(k8sNodesKubeletObjectType(), map[string]attr.Value{
		"eviction_hard_memory":   types.StringValue("7%"),
		"eviction_soft_memory":   types.StringValue("10%"),
		"kube_reserved_cpu":      types.StringValue("100m"),
		"kube_reserved_memory":   types.StringValue("512Mi"),
		"system_reserved_cpu":    types.StringValue("100m"),
		"system_reserved_memory": types.StringValue("512Mi"),
	})
}

// The top-level guard against the api module's ConfigSpec runs with no
// omissions: every key of the pool spec is modelled, so a field added upstream
// surfaces as a failing test naming it. The nested blocks get their own guards
// below, since this one reflects a single level of json tags.
func TestKubernetesNodesExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullKubernetesNodesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, kubernetesnodes.ConfigSpec{})
}

// The top-level guard reflects one level of json tags, so the talos, kubelet,
// images, and gpu submaps are four more hand-mapped surfaces it does not reach.
// Guarding each separately means a field added to one of them upstream surfaces
// as a failing test naming the field rather than going silently unmodelled.
func TestKubernetesNodesExpandNestedKeysMatchTheirSpecs(t *testing.T) {
	t.Parallel()

	model := fullKubernetesNodesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	gpus, _ := got.Spec["gpus"].([]any)
	if len(gpus) != 1 {
		t.Fatalf("gpus has %d entries, want 1", len(gpus))
	}

	tests := []struct {
		block string
		raw   any
		spec  any
	}{
		{block: "talos", raw: got.Spec["talos"], spec: kubernetesnodes.Talos{}},
		{block: "kubelet", raw: got.Spec["kubelet"], spec: kubernetesnodes.Kubelet{}},
		{block: "images", raw: got.Spec["images"], spec: kubernetesnodes.Images{}},
		{block: "gpus[0]", raw: gpus[0], spec: kubernetesnodes.GPU{}},
	}

	for _, tt := range tests {
		t.Run(tt.block, func(t *testing.T) {
			t.Parallel()

			fields, ok := tt.raw.(map[string]any)
			if !ok {
				t.Fatalf("%s = %#v, want a submap", tt.block, tt.raw)
			}

			emitted := make(map[string]bool, len(fields))
			for key := range fields {
				emitted[key] = true
			}

			assertSpecCoverage(t, emitted, tt.spec)
		})
	}
}

func TestKubernetesNodesExpand_NestedBlocks(t *testing.T) {
	t.Parallel()

	model := fullKubernetesNodesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	talos, _ := got.Spec["talos"].(map[string]any)
	if talos["schematicID"] != "ce4c9805" || talos["installerRepository"] != "registry.example.com/installer" {
		t.Errorf("talos = %v, want the configured schematic and installer repository", talos)
	}

	kubelet, _ := got.Spec["kubelet"].(map[string]any)
	if kubelet["kubeReservedCpu"] != "100m" || kubelet["evictionHardMemory"] != "7%" {
		t.Errorf("kubelet = %v, want the configured reservations", kubelet)
	}

	gpus, _ := got.Spec["gpus"].([]any)
	if len(gpus) != 1 {
		t.Fatalf("gpus has %d entries, want 1", len(gpus))
	}

	entry, _ := gpus[0].(map[string]any)
	if entry[attrName] != "nvidia.com/AD102GL_L40S" {
		t.Errorf("gpus[0].name = %v, want nvidia.com/AD102GL_L40S", entry[attrName])
	}
}

// An unset Talos or image block must leave its keys out entirely: the chart's
// own image factory, schematic, and Talos release move with the Cozystack
// release, and writing a provider-side value would pin the pool to whatever it
// happened to be when the config was written.
func TestKubernetesNodesExpand_UnsetChartBlocksStayEmpty(t *testing.T) {
	t.Parallel()

	model := fullKubernetesNodesModel()
	model.Talos = types.ObjectNull(k8sNodesTalosObjectType())
	model.Images = types.ObjectUnknown(k8sNodesImagesObjectType())
	model.Kubelet = types.ObjectNull(k8sNodesKubeletObjectType())

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	for _, key := range []string{"talos", "images", "kubelet"} {
		block, ok := got.Spec[key].(map[string]any)
		if !ok {
			t.Fatalf("%s = %#v, want an empty submap", key, got.Spec[key])
		}

		if len(block) != 0 {
			t.Errorf("%s = %v, want no keys written", key, block)
		}
	}
}

func TestKubernetesNodesExpand_UnsetListsOmitTheirKeys(t *testing.T) {
	t.Parallel()

	model := fullKubernetesNodesModel()
	model.Roles = types.ListNull(types.StringType)
	model.Gpus = types.ListUnknown(types.ObjectType{AttrTypes: vmGpuObjectType()})

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	for _, key := range []string{"roles", "gpus"} {
		if _, ok := got.Spec[key]; ok {
			t.Errorf("%s present for an unset attribute, want the key omitted", key)
		}
	}
}

// Clearing a chart-defaulted field is a deliberate opt-out, so the empty value
// has to reach the spec rather than collapsing into an omitted key.
func TestKubernetesNodesExpand_KeepsExplicitlyEmptyFields(t *testing.T) {
	t.Parallel()

	model := fullKubernetesNodesModel()
	model.Images = types.ObjectValueMust(k8sNodesImagesObjectType(), map[string]attr.Value{
		"kubectl": types.StringValue(""),
	})

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	images, _ := got.Spec["images"].(map[string]any)

	kubectl, ok := images["kubectl"]
	if !ok {
		t.Fatalf("images.kubectl absent for an explicit empty value, want it written")
	}

	if kubectl != "" {
		t.Errorf("images.kubectl = %v, want an empty string", kubectl)
	}
}

func TestKubernetesNodesFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "demo-md1",
		Namespace: "tenant-root",
		UID:       "a9f1",
		Spec: map[string]any{
			"cluster": "demo", "storageClass": "replicated", "minReplicas": int64(1),
			"maxReplicas": int64(5), "instanceType": "u1.medium", "diskSize": "20Gi",
			"roles": []any{"ingress-nginx"}, "resources": map[string]any{"cpu": "4", "memory": "8Gi"},
			"gpus":         []any{map[string]any{"name": "nvidia.com/AD102GL_L40S"}},
			"kubelet":      map[string]any{"evictionHardMemory": "7%", "evictionSoftMemory": "10%"},
			"maxUnhealthy": "50%", "nodeStartupTimeout": "10m", "version": "v1.34",
			"talos": map[string]any{
				"version": "v1.13.6", "schematicID": "ce4c9805",
				"imageFactoryURL": "https://factory.talos.dev", "installerRepository": "factory.talos.dev/installer",
			},
			"images": map[string]any{"kubectl": ""},
		},
		Status: client.ApplicationStatus{Version: "1.6.1", Ready: true},
	}

	var model kubernetesNodesModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "tenant-root/demo-md1" {
		t.Errorf("id = %q, want tenant-root/demo-md1", model.ID.ValueString())
	}

	if model.Cluster.ValueString() != "demo" || model.Version.ValueString() != "v1.34" {
		t.Errorf("cluster/version = %q/%q, want demo/v1.34", model.Cluster.ValueString(), model.Version.ValueString())
	}

	if model.MaxReplicas.ValueInt64() != 5 {
		t.Errorf("max_replicas = %d, want 5", model.MaxReplicas.ValueInt64())
	}

	if len(model.Roles.Elements()) != 1 || len(model.Gpus.Elements()) != 1 {
		t.Errorf("roles/gpus = %v/%v, want one element each", model.Roles, model.Gpus)
	}
}

// The server materialises the chart's defaults into every read, so a block the
// operator never wrote comes back carrying the fields that have defaults and
// nothing else. The ones it left out must stay null instead of turning into
// empty strings the next plan would report as drift.
func TestKubernetesNodesFlatten_UnwrittenBlockFieldsStayNull(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "demo-md1",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"kubelet": map[string]any{"evictionHardMemory": "7%", "evictionSoftMemory": "10%"},
			"images":  map[string]any{"kubectl": ""},
		},
	}

	var model kubernetesNodesModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	var kubelet k8sNodesKubeletData
	if diags := model.Kubelet.As(context.Background(), &kubelet, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("kubelet decode diagnostics: %v", diags)
	}

	if kubelet.EvictionHardMemory.ValueString() != "7%" {
		t.Errorf("eviction_hard_memory = %v, want 7%%", kubelet.EvictionHardMemory)
	}

	if !kubelet.KubeReservedCPU.IsNull() || !kubelet.SystemReservedMemory.IsNull() {
		t.Errorf("unwritten kubelet reservations = %v/%v, want null",
			kubelet.KubeReservedCPU, kubelet.SystemReservedMemory)
	}

	var images k8sNodesImagesData
	if diags := model.Images.As(context.Background(), &images, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("images decode diagnostics: %v", diags)
	}

	if images.Kubectl.IsNull() || images.Kubectl.ValueString() != "" {
		t.Errorf("images.kubectl = %v, want a stored empty string", images.Kubectl)
	}

	// Talos is absent from this spec entirely, which is the only state that
	// flattens the whole block to null.
	if !model.Talos.IsNull() {
		t.Errorf("talos = %v for an absent block, want null", model.Talos)
	}
}

// The framework binds a model to its schema by tfsdk tag, and a tag with no
// matching attribute surfaces only once Terraform runs the resource. Comparing
// the two sets here names the offending attribute instead.
func TestKubernetesNodesSchemasMatchTheirModels(t *testing.T) {
	t.Parallel()

	resourceAttributes := map[string]bool{}
	for name := range kubernetesNodesSchema().Attributes {
		resourceAttributes[name] = true
	}

	assertAttributeNames(t, "resource", resourceAttributes, kubernetesNodesResourceModel{})

	dataSourceAttributes := map[string]bool{}
	for name := range kubernetesNodesDataSourceSchema().Attributes {
		dataSourceAttributes[name] = true
	}

	assertAttributeNames(t, "data source", dataSourceAttributes, kubernetesNodesModel{})
}

func assertAttributeNames(t *testing.T, kind string, attributes map[string]bool, model any) {
	t.Helper()

	tagged := map[string]bool{}
	collectTFSDKTags(reflect.TypeOf(model), tagged)

	for name := range attributes {
		if !tagged[name] {
			t.Errorf("%s schema has %q, which no model field is tagged for", kind, name)
		}
	}

	for name := range tagged {
		if !attributes[name] {
			t.Errorf("%s model is tagged %q, which the schema has no attribute for", kind, name)
		}
	}
}

// collectTFSDKTags reflects a model's tfsdk tags, descending into the base model
// a resource model embeds.
func collectTFSDKTags(typ reflect.Type, tagged map[string]bool) {
	for i := range typ.NumField() {
		field := typ.Field(i)

		if tag := field.Tag.Get("tfsdk"); tag != "" {
			tagged[tag] = true

			continue
		}

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			collectTFSDKTags(field.Type, tagged)
		}
	}
}

// The chart derives the pool name from the release name and fails the render
// when the parent cluster is not its prefix, so the mismatch has to be caught
// before it becomes a HelmRelease that never installs.
func TestKubernetesNodesExpand_RejectsNameWithoutClusterPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		poolName string
		valid    bool
	}{
		{name: "prefixed by the cluster", poolName: "demo-md1", valid: true},
		{name: "pool name carries separators", poolName: "demo-gpu-a100", valid: true},
		{name: "unrelated name", poolName: "md1"},
		{name: "another cluster's pool", poolName: "staging-md1"},
		{name: "cluster name alone", poolName: "demo"},
		{name: "empty pool part", poolName: "demo-"},
		// A cluster whose name is a prefix of another must not pass by accident.
		{name: "prefix without the separator", poolName: "demolition-md1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := fullKubernetesNodesModel()
			model.Name = types.StringValue(tt.poolName)

			_, diags := model.expand(context.Background())
			if diags.HasError() == tt.valid {
				t.Errorf("expand of %q reported error=%v, want error=%v", tt.poolName, diags.HasError(), !tt.valid)
			}
		})
	}
}

// unsetKubernetesNodesModel is a pool as it arrives off a plan that configured
// none of the chart-owned blocks: the attributes carrying a schema default hold
// it, everything else is a typed null.
func unsetKubernetesNodesModel() kubernetesNodesModel {
	return kubernetesNodesModel{
		Name:               types.StringValue("demo-md1"),
		Namespace:          types.StringValue("tenant-root"),
		Cluster:            types.StringValue("demo"),
		StorageClass:       types.StringValue("replicated"),
		MinReplicas:        types.Int64Value(0),
		MaxReplicas:        types.Int64Value(10),
		InstanceType:       types.StringValue("u1.medium"),
		DiskSize:           types.StringValue("20Gi"),
		Roles:              types.ListNull(types.StringType),
		Resources:          types.ObjectNull(resourcesObjectType()),
		Gpus:               types.ListNull(types.ObjectType{AttrTypes: vmGpuObjectType()}),
		Kubelet:            types.ObjectNull(k8sNodesKubeletObjectType()),
		MaxUnhealthy:       types.StringValue("50%"),
		NodeStartupTimeout: types.StringValue("10m"),
		Version:            types.StringValue("v1.35"),
		Talos:              types.ObjectNull(k8sNodesTalosObjectType()),
		Images:             types.ObjectNull(k8sNodesImagesObjectType()),
	}
}

// serverResolvedApplication is what a GET answers with for that pool: the
// aggregated API materialises the chart's schema defaults into the spec on every
// read, so blocks the pool never configured come back populated.
func serverResolvedApplication() *client.Application {
	return &client.Application{
		Name:      "demo-md1",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"cluster": "demo", "storageClass": "replicated", "minReplicas": int64(0),
			"maxReplicas": int64(10), "instanceType": "u1.medium", "diskSize": "20Gi",
			"roles": []any{}, "resources": map[string]any{}, "gpus": []any{},
			"kubelet":      map[string]any{"evictionHardMemory": "7%", "evictionSoftMemory": "10%"},
			"maxUnhealthy": "50%", "nodeStartupTimeout": "10m", "version": "v1.35",
			"talos": map[string]any{
				"version": "v1.13.6", "schematicID": "ce4c9805",
				"imageFactoryURL": "https://factory.talos.dev", "installerRepository": "factory.talos.dev/installer",
			},
			"images": map[string]any{"kubectl": ""},
		},
	}
}

// The round trip that matters for a chart-owned block is flatten followed by
// expand, and no single-direction test covers it. Taking the values the server
// resolved into state would make the next update write them back as explicit
// spec keys, pinning the pool to the Talos release and images that were current
// the day it was created while its parent cluster keeps following the chart.
func TestKubernetesNodesResourceFlatten_DoesNotPinChartDefaults(t *testing.T) {
	t.Parallel()

	model := kubernetesNodesResourceModel{kubernetesNodesModel: unsetKubernetesNodesModel()}
	model.ID = types.StringValue("tenant-root/demo-md1")

	if diags := model.flatten(serverResolvedApplication()); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !model.Talos.IsNull() || !model.Images.IsNull() || !model.Kubelet.IsNull() {
		t.Errorf("chart blocks reached state as talos=%v images=%v kubelet=%v, want null",
			model.Talos, model.Images, model.Kubelet)
	}

	if !model.Roles.IsNull() || !model.Gpus.IsNull() {
		t.Errorf("unset lists reached state as roles=%v gpus=%v, want null", model.Roles, model.Gpus)
	}

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	for _, key := range []string{"talos", "kubelet", "images"} {
		block, _ := got.Spec[key].(map[string]any)
		if len(block) != 0 {
			t.Errorf("%s = %v on the update after a read, want the chart's values not written back", key, block)
		}
	}
}

// A block the pool configures partially keeps exactly what it set: the field it
// wrote round-trips, the siblings the server resolved for it do not.
func TestKubernetesNodesResourceFlatten_KeepsConfiguredFieldsOnly(t *testing.T) {
	t.Parallel()

	base := unsetKubernetesNodesModel()
	base.Talos = types.ObjectValueMust(k8sNodesTalosObjectType(), map[string]attr.Value{
		"version":              types.StringNull(),
		"schematic_id":         types.StringValue("custom"),
		"image_factory_url":    types.StringNull(),
		"installer_repository": types.StringNull(),
	})

	model := kubernetesNodesResourceModel{kubernetesNodesModel: base}
	model.ID = types.StringValue("tenant-root/demo-md1")

	app := serverResolvedApplication()
	app.Spec["talos"].(map[string]any)["schematicID"] = "custom"

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	var talos k8sNodesTalosData
	if diags := model.Talos.As(context.Background(), &talos, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("talos decode diagnostics: %v", diags)
	}

	if talos.SchematicID.ValueString() != "custom" {
		t.Errorf("schematic_id = %v, want the configured value", talos.SchematicID)
	}

	if !talos.Version.IsNull() || !talos.ImageFactoryURL.IsNull() || !talos.InstallerRepository.IsNull() {
		t.Errorf("unconfigured talos fields = %v/%v/%v, want null",
			talos.Version, talos.ImageFactoryURL, talos.InstallerRepository)
	}

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted, _ := got.Spec["talos"].(map[string]any)
	if len(emitted) != 1 || emitted["schematicID"] != "custom" {
		t.Errorf("talos = %v, want only the configured schematicID", emitted)
	}
}

// The read that follows terraform import is the only chance to capture what a
// pool already has: state carries nothing but the identity, so masking there
// would hide an existing override from the plan and let the next update, which
// replaces the spec whole, silently delete it.
func TestKubernetesNodesResourceFlatten_ImportCapturesServerState(t *testing.T) {
	t.Parallel()

	app := serverResolvedApplication()
	app.Spec["roles"] = []any{"ingress-nginx"}
	app.Spec["gpus"] = []any{map[string]any{"name": "nvidia.com/AD102GL_L40S"}}
	app.Spec["talos"].(map[string]any)["installerRepository"] = "airgap.example.com/installer"

	// ImportState writes only the identity, leaving every other attribute null.
	var model kubernetesNodesResourceModel

	model.Name = types.StringValue("demo-md1")
	model.Namespace = types.StringValue("tenant-root")

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if len(model.Roles.Elements()) != 1 || len(model.Gpus.Elements()) != 1 {
		t.Errorf("roles/gpus = %v/%v after import, want the server's values", model.Roles, model.Gpus)
	}

	var talos k8sNodesTalosData
	if diags := model.Talos.As(context.Background(), &talos, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("talos decode diagnostics: %v", diags)
	}

	if talos.InstallerRepository.ValueString() != "airgap.example.com/installer" {
		t.Errorf("talos.installer_repository = %v after import, want the server's override", talos.InstallerRepository)
	}

	if model.Kubelet.IsNull() || model.Images.IsNull() {
		t.Errorf("kubelet/images = %v/%v after import, want the server's values", model.Kubelet, model.Images)
	}
}

// An imported pool whose lists the server reports as empty keeps them null, so
// the plan right after the import does not open with a diff that removes two
// lists nobody ever set.
func TestKubernetesNodesResourceFlatten_ImportLeavesEmptyListsNull(t *testing.T) {
	t.Parallel()

	var model kubernetesNodesResourceModel

	model.Name = types.StringValue("demo-md1")
	model.Namespace = types.StringValue("tenant-root")

	if diags := model.flatten(serverResolvedApplication()); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !model.Roles.IsNull() || !model.Gpus.IsNull() {
		t.Errorf("roles/gpus = %v/%v, want null for lists the server reports empty", model.Roles, model.Gpus)
	}
}

// A name or cluster that another resource supplies is still unknown while the
// plan is built. Rejecting it there would fail configurations that are correct,
// so the unknown case is deferred to the apply-time call in expand.
func TestKubernetesNodesValidateConfig_SkipsUnknownValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*kubernetesNodesModel)
		wantErr bool
	}{
		{
			name:   "unknown name",
			mutate: func(m *kubernetesNodesModel) { m.Name = types.StringUnknown() },
		},
		{
			name:   "unknown cluster",
			mutate: func(m *kubernetesNodesModel) { m.Cluster = types.StringUnknown() },
		},
		{
			name:    "both known and mismatched",
			mutate:  func(m *kubernetesNodesModel) { m.Name = types.StringValue("staging-md1") },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := fullKubernetesNodesModel()
			tt.mutate(&model)

			if got := model.validateConfig().HasError(); got != tt.wantErr {
				t.Errorf("validateConfig error = %v, want %v", got, tt.wantErr)
			}
		})
	}
}

// A field the server does not answer with keeps what the configuration set,
// rather than being dropped from the merged block.
func TestKubernetesNodesResourceFlatten_KeepsConfiguredFieldsTheServerDrops(t *testing.T) {
	t.Parallel()

	base := unsetKubernetesNodesModel()
	base.Images = types.ObjectValueMust(k8sNodesImagesObjectType(), map[string]attr.Value{
		"kubectl": types.StringValue("registry.example.com/kubectl:v1.35.0"),
	})

	model := kubernetesNodesResourceModel{kubernetesNodesModel: base}
	model.ID = types.StringValue("tenant-root/demo-md1")

	app := serverResolvedApplication()
	delete(app.Spec, "images")

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	var images k8sNodesImagesData
	if diags := model.Images.As(context.Background(), &images, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("images decode diagnostics: %v", diags)
	}

	if images.Kubectl.ValueString() != "registry.example.com/kubectl:v1.35.0" {
		t.Errorf("images.kubectl = %v, want the configured value kept", images.Kubectl)
	}
}
