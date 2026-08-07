package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/kubernetes"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullTalosObject() types.Object {
	return types.ObjectValueMust(k8sTalosObjectType(), map[string]attr.Value{
		"image_factory_url":    types.StringValue("https://factory.example.test"),
		"installer_repository": types.StringValue("registry.example.test/installer"),
		"schematic_id":         types.StringValue("ce4c980550dd2ab1b17bbf2b08801c7eb59418eafe8f279833297925d67c7515"),
		"version":              types.StringValue("v1.13.6"),
	})
}

// fullOIDCObject sets every OIDC key so the coverage guards see the whole
// surface. An inline config and a secretRef are mutually exclusive in practice;
// the schema validators reject that pairing, expand does not police it.
func fullOIDCObject() types.Object {
	user := types.ObjectValueMust(k8sOIDCUserObjectType(), map[string]attr.Value{
		"email": types.StringValue("operator@example.test"),
		"role":  types.StringValue("admin"),
	})

	secretRef := types.ObjectValueMust(k8sOIDCSecretRefObjectType(), map[string]attr.Value{
		attrName: types.StringValue("tenant-authentication-config"),
	})

	customConfig := types.ObjectValueMust(k8sOIDCCustomConfigObjectType(), map[string]attr.Value{
		"config":     types.StringValue("apiVersion: apiserver.config.k8s.io/v1beta1\n"),
		"secret_ref": secretRef,
	})

	return types.ObjectValueMust(k8sOIDCObjectType(), map[string]attr.Value{
		"mode":          types.StringValue("System"),
		"users":         types.ListValueMust(types.ObjectType{AttrTypes: k8sOIDCUserObjectType()}, []attr.Value{user}),
		"custom_config": customConfig,
	})
}

func fullControlPlaneObject() types.Object {
	apiServer := types.ObjectValueMust(k8sAPIServerObjectType(), map[string]attr.Value{
		"extra_args": types.ListValueMust(types.StringType, []attr.Value{
			types.StringValue("--requestheader-uid-headers=X-Remote-Uid"),
		}),
		"extra_volumes": types.ListValueMust(jsontypes.NormalizedType{}, []attr.Value{
			jsontypes.NewNormalizedValue(`{"name":"auth-config","configMap":{"name":"auth-config"}}`),
		}),
		"extra_volume_mounts": types.ListValueMust(jsontypes.NormalizedType{}, []attr.Value{
			jsontypes.NewNormalizedValue(`{"name":"auth-config","mountPath":"/etc/kubernetes/auth"}`),
		}),
	})

	return types.ObjectValueMust(k8sControlPlaneObjectType(), map[string]attr.Value{
		"api_server": apiServer,
	})
}

func fullKubernetesModel() kubernetesModel {
	group := types.ObjectValueMust(k8sNodeGroupObjectType(), map[string]attr.Value{
		"disk_size":            types.StringValue("20Gi"),
		"instance_type":        types.StringValue("u1.medium"),
		"min_replicas":         types.Int64Value(0),
		"max_replicas":         types.Int64Value(3),
		"roles":                types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ingress-nginx")}),
		"storage_class":        types.StringValue(""),
		"resources":            types.ObjectNull(resourcesObjectType()),
		"max_unhealthy":        types.StringValue("0%"),
		"node_startup_timeout": types.StringValue("20m"),
	})

	return kubernetesModel{
		Name:         types.StringValue("cluster"),
		Namespace:    types.StringValue("tenant-root"),
		StorageClass: types.StringValue("replicated"),
		Version:      types.StringValue("v1.35"),
		Host:         types.StringValue("cluster.example.com"),
		NodeGroups:   types.MapValueMust(types.ObjectType{AttrTypes: k8sNodeGroupObjectType()}, map[string]attr.Value{"md0": group}),
		Talos:        fullTalosObject(),
		NodeHealthCheck: types.ObjectValueMust(k8sNodeHealthCheckObjectType(), map[string]attr.Value{
			"max_unhealthy":        types.StringValue("50%"),
			"node_startup_timeout": types.StringValue("10m"),
		}),
		OIDC:         fullOIDCObject(),
		ControlPlane: fullControlPlaneObject(),
		Images: types.ObjectValueMust(k8sImagesObjectType(), map[string]attr.Value{
			"kubectl":             types.StringValue("registry.example.test/kubectl:v1.35.0"),
			"talos_csr_signer":    types.StringValue("registry.example.test/talos-csr-signer:v0.1.0"),
			"wait_for_kubeconfig": types.StringValue("registry.example.test/busybox:1.37"),
		}),
	}
}

// nestedSpec walks into a nested block of the emitted spec.
func nestedSpec(t *testing.T, spec map[string]any, path ...string) map[string]any {
	t.Helper()

	current := spec

	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			t.Fatalf("spec block %q missing from the emitted spec", strings.Join(path, "."))
		}

		current = next
	}

	return current
}

// nestedSpecKeys returns the keys of a nested block, so the coverage guard can
// be pointed at a struct below the top level.
func nestedSpecKeys(t *testing.T, spec map[string]any, path ...string) map[string]bool {
	t.Helper()

	block := nestedSpec(t, spec, path...)

	keys := make(map[string]bool, len(block))
	for key := range block {
		keys[key] = true
	}

	return keys
}

func TestKubernetesExpand_NodeGroups(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	groups, _ := got.Spec["nodeGroups"].(map[string]any)
	md0, _ := groups["md0"].(map[string]any)
	if md0["maxReplicas"] != int64(3) {
		t.Errorf("md0.maxReplicas = %v, want 3", md0["maxReplicas"])
	}

	roles, _ := md0["roles"].([]any)
	if len(roles) != 1 || roles[0] != "ingress-nginx" {
		t.Errorf("md0.roles = %v, want [ingress-nginx]", roles)
	}
}

// The per-group health-check overrides have no upstream default: an absent key
// means "inherit the cluster-wide nodeHealthCheck", so they must not be written
// as empty strings.
func TestKubernetesExpand_NodeGroupHealthOverrides(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	md0 := nestedSpecKeys(t, got.Spec, "nodeGroups", "md0")
	if !md0["maxUnhealthy"] || !md0["nodeStartupTimeout"] {
		t.Errorf("node group keys = %v, want the health-check overrides emitted", md0)
	}

	bare := types.ObjectValueMust(k8sNodeGroupObjectType(), map[string]attr.Value{
		"disk_size":            types.StringValue("20Gi"),
		"instance_type":        types.StringValue("u1.medium"),
		"min_replicas":         types.Int64Value(0),
		"max_replicas":         types.Int64Value(3),
		"roles":                types.ListNull(types.StringType),
		"storage_class":        types.StringValue(""),
		"resources":            types.ObjectNull(resourcesObjectType()),
		"max_unhealthy":        types.StringNull(),
		"node_startup_timeout": types.StringNull(),
	})

	model.NodeGroups = types.MapValueMust(
		types.ObjectType{AttrTypes: k8sNodeGroupObjectType()},
		map[string]attr.Value{"md0": bare},
	)

	got, diags = model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	md0 = nestedSpecKeys(t, got.Spec, "nodeGroups", "md0")
	if md0["maxUnhealthy"] || md0["nodeStartupTimeout"] {
		t.Errorf("node group keys = %v, want the unset health-check overrides omitted", md0)
	}
}

func TestKubernetesFlatten_NodeGroupHealthOverrides(t *testing.T) {
	t.Parallel()

	groups, diags := flattenNodeGroups(map[string]any{
		"md0": map[string]any{"maxUnhealthy": "1"},
	})
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	md0, _ := groups.Elements()["md0"].(types.Object)

	maxUnhealthy, _ := md0.Attributes()["max_unhealthy"].(types.String)
	if maxUnhealthy.ValueString() != "1" {
		t.Errorf("max_unhealthy = %q, want 1", maxUnhealthy.ValueString())
	}

	timeout, _ := md0.Attributes()["node_startup_timeout"].(types.String)
	if !timeout.IsNull() {
		t.Errorf("node_startup_timeout = %v for an absent key, want null", timeout)
	}
}

func TestKubernetesExpand_OmitsEmptyHost(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.Host = types.StringValue("")

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if _, ok := got.Spec[attrHost]; ok {
		t.Errorf("host key present for empty host, want omitted")
	}
}

// The talos block carries the worker OS image coordinates. Its upstream
// defaults roll with every Cozystack release, so an unset block must leave the
// key out of the spec entirely and let the server supply the current value.
func TestKubernetesExpand_TalosOmittedWhenUnset(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.Talos = types.ObjectNull(k8sTalosObjectType())

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if _, ok := got.Spec["talos"]; ok {
		t.Errorf("talos key present for an unset block, want omitted")
	}
}

func TestKubernetesExpand_TalosEmitsOnlySetFields(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.Talos = types.ObjectValueMust(k8sTalosObjectType(), map[string]attr.Value{
		"image_factory_url":    types.StringNull(),
		"installer_repository": types.StringNull(),
		"schematic_id":         types.StringNull(),
		"version":              types.StringValue("v1.13.7"),
	})

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	talos, ok := got.Spec["talos"].(map[string]any)
	if !ok {
		t.Fatalf("talos = %#v, want a spec submap", got.Spec["talos"])
	}

	if len(talos) != 1 || talos["version"] != "v1.13.7" {
		t.Errorf("talos = %v, want only version=v1.13.7", talos)
	}
}

func TestKubernetesFlatten_Talos(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		spec  map[string]any
		null  bool
		field string
		want  string
	}{
		{name: "absent block flattens to null", spec: map[string]any{}, null: true},
		{
			name:  "server-defaulted block round-trips",
			spec:  map[string]any{"talos": map[string]any{"version": "v1.13.6"}},
			field: "version",
			want:  "v1.13.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := flattenTalos(tt.spec["talos"])
			if got.IsNull() != tt.null {
				t.Fatalf("talos null = %v, want %v", got.IsNull(), tt.null)
			}

			if tt.null {
				return
			}

			value, _ := got.Attributes()[tt.field].(types.String)
			if value.ValueString() != tt.want {
				t.Errorf("talos.%s = %q, want %q", tt.field, value.ValueString(), tt.want)
			}
		})
	}
}

func TestKubernetesExpandTalosKeysMatchTalosSpec(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	assertSpecCoverage(t, nestedSpecKeys(t, got.Spec, "talos"), kubernetes.Talos{})
}

// nodeHealthCheck tunes MachineHealthCheck remediation. An unset block must
// stay out of the spec so the platform's own tuning applies.
func TestKubernetesExpand_NodeHealthCheck(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.NodeHealthCheck = types.ObjectNull(k8sNodeHealthCheckObjectType())

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if _, ok := got.Spec["nodeHealthCheck"]; ok {
		t.Fatalf("nodeHealthCheck key present for an unset block, want omitted")
	}

	model = fullKubernetesModel()

	got, diags = model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	check, _ := got.Spec["nodeHealthCheck"].(map[string]any)
	if check["maxUnhealthy"] != "50%" || check["nodeStartupTimeout"] != "10m" {
		t.Errorf("nodeHealthCheck = %v, want maxUnhealthy=50%% nodeStartupTimeout=10m", check)
	}
}

func TestKubernetesFlatten_NodeHealthCheck(t *testing.T) {
	t.Parallel()

	if got := flattenNodeHealthCheck(nil); !got.IsNull() {
		t.Errorf("nodeHealthCheck = %v for an absent key, want null", got)
	}

	got := flattenNodeHealthCheck(map[string]any{"maxUnhealthy": "0%", "nodeStartupTimeout": "20m"})
	if got.IsNull() {
		t.Fatalf("nodeHealthCheck is null for a populated block")
	}

	timeout, _ := got.Attributes()["node_startup_timeout"].(types.String)
	if timeout.ValueString() != "20m" {
		t.Errorf("node_startup_timeout = %q, want 20m", timeout.ValueString())
	}
}

func TestKubernetesExpandNodeHealthCheckKeysMatchSpec(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	assertSpecCoverage(t, nestedSpecKeys(t, got.Spec, "nodeHealthCheck"), kubernetes.NodeHealthCheck{})
}

func TestKubernetesExpand_OIDCOmittedWhenUnset(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.OIDC = types.ObjectNull(k8sOIDCObjectType())

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if _, ok := got.Spec["oidc"]; ok {
		t.Errorf("oidc key present for an unset block, want omitted")
	}
}

func TestKubernetesExpand_OIDCUsersAndCustomConfig(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	oidc, _ := got.Spec["oidc"].(map[string]any)
	if oidc["mode"] != "System" {
		t.Errorf("oidc.mode = %v, want System", oidc["mode"])
	}

	users, _ := oidc["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("oidc.users has %d entries, want 1", len(users))
	}

	user, _ := users[0].(map[string]any)
	if user["email"] != "operator@example.test" || user["role"] != "admin" {
		t.Errorf("oidc.users[0] = %v, want the admin operator entry", user)
	}

	custom, _ := oidc["customConfig"].(map[string]any)

	secretRef, _ := custom["secretRef"].(map[string]any)
	if secretRef["name"] != "tenant-authentication-config" {
		t.Errorf("oidc.customConfig.secretRef.name = %v, want tenant-authentication-config", secretRef["name"])
	}
}

// An empty users list is not the same as no users list: the platform defaults
// the key to an empty list, so a practitioner who writes `users = []` is opting
// out of user bindings explicitly and the key must be emitted as written.
func TestKubernetesExpand_OIDCEmptyUsersListIsEmitted(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.OIDC = types.ObjectValueMust(k8sOIDCObjectType(), map[string]attr.Value{
		"mode":          types.StringValue("None"),
		"users":         types.ListValueMust(types.ObjectType{AttrTypes: k8sOIDCUserObjectType()}, []attr.Value{}),
		"custom_config": types.ObjectNull(k8sOIDCCustomConfigObjectType()),
	})

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	oidc, _ := got.Spec["oidc"].(map[string]any)

	users, ok := oidc["users"].([]any)
	if !ok || len(users) != 0 {
		t.Errorf("oidc.users = %#v, want an empty list", oidc["users"])
	}

	if _, ok := oidc["customConfig"]; ok {
		t.Errorf("oidc.customConfig present for an unset block, want omitted")
	}
}

func TestKubernetesFlatten_OIDC(t *testing.T) {
	t.Parallel()

	null, diags := flattenOIDC(nil)
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !null.IsNull() {
		t.Errorf("oidc = %v for an absent key, want null", null)
	}

	got, diags := flattenOIDC(map[string]any{
		"mode":         "System",
		"users":        []any{map[string]any{"email": "operator@example.test", "role": "view"}},
		"customConfig": map[string]any{"config": "", "secretRef": map[string]any{"name": ""}},
	})
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	users, _ := got.Attributes()["users"].(types.List)
	if len(users.Elements()) != 1 {
		t.Fatalf("oidc.users has %d elements, want 1", len(users.Elements()))
	}

	custom, _ := got.Attributes()["custom_config"].(types.Object)

	config, _ := custom.Attributes()["config"].(types.String)
	if config.IsNull() {
		t.Errorf("custom_config.config is null, want the server's empty string")
	}
}

func TestKubernetesExpandOIDCKeysMatchOIDCSpec(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	assertSpecCoverage(t, nestedSpecKeys(t, got.Spec, "oidc"), kubernetes.OIDC{})
	assertSpecCoverage(t, nestedSpecKeys(t, got.Spec, "oidc", "customConfig"), kubernetes.OIDCCustomConfig{})
	assertSpecCoverage(t, nestedSpecKeys(t, got.Spec, "oidc", "customConfig", "secretRef"), kubernetes.OIDCSecretRef{})

	oidc, _ := got.Spec["oidc"].(map[string]any)
	users, _ := oidc["users"].([]any)
	user, _ := users[0].(map[string]any)

	emitted := make(map[string]bool, len(user))
	for key := range user {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, kubernetes.OIDCUser{})
}

// controlPlane is a passthrough to the KamajiControlPlane. Everything except
// apiServer stays server-defaulted, so an unset block must not appear.
func TestKubernetesExpand_ControlPlane(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.ControlPlane = types.ObjectNull(k8sControlPlaneObjectType())

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if _, ok := got.Spec["controlPlane"]; ok {
		t.Fatalf("controlPlane key present for an unset block, want omitted")
	}

	model = fullKubernetesModel()

	got, diags = model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	apiServer := nestedSpec(t, got.Spec, "controlPlane", "apiServer")

	args, _ := apiServer["extraArgs"].([]any)
	if len(args) != 1 || args[0] != "--requestheader-uid-headers=X-Remote-Uid" {
		t.Errorf("apiServer.extraArgs = %v, want the single passthrough flag", args)
	}

	volumes, _ := apiServer["extraVolumes"].([]any)

	volume, _ := volumes[0].(map[string]any)
	if volume["name"] != "auth-config" {
		t.Errorf("apiServer.extraVolumes[0] = %v, want the auth-config volume", volume)
	}
}

func TestKubernetesFlatten_ControlPlane(t *testing.T) {
	t.Parallel()

	null, diags := flattenControlPlane(nil)
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !null.IsNull() {
		t.Errorf("control_plane = %v for an absent key, want null", null)
	}

	got, diags := flattenControlPlane(map[string]any{
		"apiServer": map[string]any{
			"extraArgs":         []any{},
			"extraVolumes":      []any{map[string]any{"name": "auth-config"}},
			"extraVolumeMounts": []any{},
		},
	})
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	apiServer, _ := got.Attributes()["api_server"].(types.Object)

	// The platform defaults these to empty lists, so an empty list read back is
	// a real value and must not collapse to null.
	args, _ := apiServer.Attributes()["extra_args"].(types.List)
	if args.IsNull() {
		t.Errorf("extra_args = null for a stored empty list, want an empty list")
	}

	volumes, _ := apiServer.Attributes()["extra_volumes"].(types.List)
	if len(volumes.Elements()) != 1 {
		t.Fatalf("extra_volumes has %d elements, want 1", len(volumes.Elements()))
	}

	encoded, _ := volumes.Elements()[0].(jsontypes.Normalized)
	if encoded.ValueString() != `{"name":"auth-config"}` {
		t.Errorf("extra_volumes[0] = %s, want the auth-config volume as JSON", encoded.ValueString())
	}
}

func TestKubernetesExpandControlPlaneKeysMatchSpec(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	// The sizing of the control-plane components stays with the server, as it
	// did before controlPlane was modelled at all.
	assertSpecCoverage(t, nestedSpecKeys(t, got.Spec, "controlPlane"), kubernetes.ControlPlane{},
		"controllerManager", "konnectivity", "replicas", "scheduler")
	assertSpecCoverage(t, nestedSpecKeys(t, got.Spec, "controlPlane", "apiServer"), kubernetes.APIServer{},
		"resources", "resourcesPreset")
}

// The image overrides exist for air-gapped and rate-limited registries. Empty
// means "use the chart's pinned tag", so an unset attribute must not be written
// as an empty string either — the key simply stays out.
func TestKubernetesExpand_Images(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.Images = types.ObjectNull(k8sImagesObjectType())

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if _, ok := got.Spec["images"]; ok {
		t.Fatalf("images key present for an unset block, want omitted")
	}

	model = fullKubernetesModel()
	model.Images = types.ObjectValueMust(k8sImagesObjectType(), map[string]attr.Value{
		"kubectl":             types.StringValue("registry.example.test/kubectl:v1.35.0"),
		"talos_csr_signer":    types.StringNull(),
		"wait_for_kubeconfig": types.StringNull(),
	})

	got, diags = model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	images := nestedSpec(t, got.Spec, "images")
	if len(images) != 1 || images["kubectl"] != "registry.example.test/kubectl:v1.35.0" {
		t.Errorf("images = %v, want only the kubectl override", images)
	}
}

func TestKubernetesFlatten_Images(t *testing.T) {
	t.Parallel()

	if got := flattenImages(nil); !got.IsNull() {
		t.Errorf("images = %v for an absent key, want null", got)
	}

	got := flattenImages(map[string]any{"kubectl": "", "talosCsrSigner": "registry.example.test/signer:v1"})

	kubectl, _ := got.Attributes()["kubectl"].(types.String)
	if kubectl.IsNull() || kubectl.ValueString() != "" {
		t.Errorf("kubectl = %v, want the server's empty string", kubectl)
	}

	waitFor, _ := got.Attributes()["wait_for_kubeconfig"].(types.String)
	if !waitFor.IsNull() {
		t.Errorf("wait_for_kubeconfig = %v for an absent key, want null", waitFor)
	}
}

func TestKubernetesExpandImagesKeysMatchSpec(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	assertSpecCoverage(t, nestedSpecKeys(t, got.Spec, "images"), kubernetes.Images{})
}

func TestKubernetesFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "cluster",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"storageClass": "replicated",
			"version":      "v1.34",
			"host":         "cluster.example.com",
			"nodeGroups": map[string]any{
				"md0": map[string]any{
					"diskSize": "20Gi", "instanceType": "u1.large", "minReplicas": int64(1),
					"maxReplicas": int64(5), "roles": []any{"ingress-nginx"}, "storageClass": "",
					"resources": map[string]any{},
				},
			},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model kubernetesModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Version.ValueString() != "v1.34" {
		t.Errorf("version = %q, want v1.34", model.Version.ValueString())
	}

	groups := model.NodeGroups.Elements()
	if _, ok := groups["md0"]; !ok {
		t.Errorf("node_groups missing md0")
	}
}

func TestKubernetesExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, kubernetes.ConfigSpec{}, "addons")
}

// The node-group spec is a second schema surface the top-level ConfigSpec guard
// does not reach, since it only reflects one level of json tags. Guarding it
// separately means a node-group field added upstream surfaces as a failing test
// naming the field, rather than silently going unmodelled.
func TestKubernetesExpandNodeGroupKeysMatchNodeGroupSpec(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	groups, _ := got.Spec["nodeGroups"].(map[string]any)

	md0, ok := groups["md0"].(map[string]any)
	if !ok {
		t.Fatalf("nodeGroups.md0 missing from the emitted spec")
	}

	emitted := make(map[string]bool, len(md0))
	for key := range md0 {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, kubernetes.NodeGroup{}, "gpus", "kubelet")
}

// The gap that made the unset-block contract hold only on the first apply:
// Terraform copies the prior state into the plan for every Optional+Computed
// attribute the configuration leaves out, and the state holds whatever the
// server materialised on the previous read. Expanding the plan alone therefore
// writes the platform's own defaults back as though they had been asked for,
// and from then on the release carries them and stops following the platform.
// applyConfig closes it by taking the configuration as the authority.
func TestKubernetesApplyConfig_DropsBlocksTheConfigNeverSet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// The configuration sets none of the blocks; the plan carries them all,
	// exactly as it would after a create against a 1.6 server.
	bare := fullKubernetesModel()
	bare.Talos = types.ObjectNull(k8sTalosObjectType())
	bare.NodeHealthCheck = types.ObjectNull(k8sNodeHealthCheckObjectType())
	bare.OIDC = types.ObjectNull(k8sOIDCObjectType())
	bare.ControlPlane = types.ObjectNull(k8sControlPlaneObjectType())
	bare.Images = types.ObjectNull(k8sImagesObjectType())

	planned := fullKubernetesModel()

	if diags := planned.applyConfig(ctx, kubernetesConfig(ctx, t, bare)); diags.HasError() {
		t.Fatalf("applyConfig diagnostics: %v", diags)
	}

	got, diags := planned.expand(ctx)
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	for _, key := range []string{"talos", "nodeHealthCheck", "oidc", "controlPlane", "images"} {
		if _, ok := got.Spec[key]; ok {
			t.Errorf("%s present after a plan that only echoed the server's own defaults, want omitted", key)
		}
	}
}

// The same hole one level down: a configuration that pins a single field gets a
// plan whose siblings are filled in from state, and writing those back pins them
// too.
func TestKubernetesApplyConfig_KeepsOnlyConfiguredFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	partial := fullKubernetesModel()
	partial.Talos = types.ObjectValueMust(k8sTalosObjectType(), map[string]attr.Value{
		"image_factory_url":    types.StringNull(),
		"installer_repository": types.StringNull(),
		"schematic_id":         types.StringNull(),
		"version":              types.StringValue("v1.13.7"),
	})
	partial.NodeHealthCheck = types.ObjectNull(k8sNodeHealthCheckObjectType())
	partial.OIDC = types.ObjectNull(k8sOIDCObjectType())
	partial.ControlPlane = types.ObjectNull(k8sControlPlaneObjectType())
	partial.Images = types.ObjectNull(k8sImagesObjectType())

	planned := fullKubernetesModel()

	if diags := planned.applyConfig(ctx, kubernetesConfig(ctx, t, partial)); diags.HasError() {
		t.Fatalf("applyConfig diagnostics: %v", diags)
	}

	got, diags := planned.expand(ctx)
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	talos := nestedSpec(t, got.Spec, "talos")
	if len(talos) != 1 || talos["version"] != "v1.13.7" {
		t.Errorf("talos = %v, want only the configured version", talos)
	}
}

// Attributes outside applyConfig's remit keep expanding from the plan, where
// their materialised defaults live.
func TestKubernetesApplyConfig_LeavesPlannedDefaultsAlone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	config := fullKubernetesModel()
	config.StorageClass = types.StringNull()

	planned := fullKubernetesModel()

	if diags := planned.applyConfig(ctx, kubernetesConfig(ctx, t, config)); diags.HasError() {
		t.Fatalf("applyConfig diagnostics: %v", diags)
	}

	got, diags := planned.expand(ctx)
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec[specStorageClass] != "replicated" {
		t.Errorf("storageClass = %v, want the planned default replicated", got.Spec[specStorageClass])
	}
}
