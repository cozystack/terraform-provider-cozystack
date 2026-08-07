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
			jsontypes.NewNormalizedValue(`{"configMap":{"name":"auth-config"},"name":"auth-config"}`),
		}),
		"extra_volume_mounts": types.ListValueMust(jsontypes.NormalizedType{}, []attr.Value{
			jsontypes.NewNormalizedValue(`{"mountPath":"/etc/kubernetes/auth","name":"auth-config"}`),
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

// The managed resource must not absorb blocks the configuration never wrote.
// The aggregated apiserver materialises this Kind's schema defaults on every
// read, so a resource that stored them would carry them into the plan and, on
// the next update, write them into the release as explicit values — freezing the
// cluster on the Talos release and schematic that were current that day. The
// data source keeps reporting everything; only the resource trims.
func TestKubernetesResourceFlatten_KeepsUnconfiguredBlocksOut(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "cluster",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"storageClass": "replicated",
			"version":      "v1.35",
			"nodeGroups":   map[string]any{},
			// The whole surface the server materialises for a cluster whose
			// configuration pinned none of it.
			"talos": map[string]any{
				"imageFactoryURL":     "https://factory.talos.dev",
				"installerRepository": "factory.talos.dev/installer",
				"schematicID":         "ce4c980550dd2ab1b17bbf2b08801c7eb59418eafe8f279833297925d67c7515",
				"version":             "v1.13.6",
			},
			"nodeHealthCheck": map[string]any{"maxUnhealthy": "50%", "nodeStartupTimeout": "10m"},
			"oidc":            map[string]any{"mode": "None", "users": []any{}},
			"images":          map[string]any{"kubectl": "", "talosCsrSigner": "", "waitForKubeconfig": ""},
			"controlPlane":    map[string]any{"apiServer": map[string]any{"extraArgs": []any{}}},
		},
	}

	var resourceModel kubernetesResourceModel

	if diags := resourceModel.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	for name, value := range map[string]types.Object{
		"talos":             resourceModel.Talos,
		"node_health_check": resourceModel.NodeHealthCheck,
		"oidc":              resourceModel.OIDC,
		"control_plane":     resourceModel.ControlPlane,
		"images":            resourceModel.Images,
	} {
		if !value.IsNull() {
			t.Errorf("%s = %v after a read of an unconfigured cluster, want null", name, value)
		}
	}

	// Same server response through the data source model, which is what the
	// effective values are for.
	var dataSourceModel kubernetesModel

	if diags := dataSourceModel.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	talos, _ := dataSourceModel.Talos.Attributes()[attrVersion].(types.String)
	if talos.ValueString() != "v1.13.6" {
		t.Errorf("data source talos.version = %q, want the platform's v1.13.6", talos.ValueString())
	}
}

// A configured field still refreshes, so drift against it stays visible; its
// unconfigured siblings stay null rather than being adopted from the server.
func TestKubernetesResourceFlatten_RefreshesConfiguredFieldsOnly(t *testing.T) {
	t.Parallel()

	resourceModel := kubernetesResourceModel{kubernetesModel: kubernetesModel{
		Talos: types.ObjectValueMust(k8sTalosObjectType(), map[string]attr.Value{
			"image_factory_url":    types.StringNull(),
			"installer_repository": types.StringNull(),
			"schematic_id":         types.StringNull(),
			attrVersion:            types.StringValue("v1.13.6"),
		}),
	}}

	app := &client.Application{
		Name:      "cluster",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"nodeGroups": map[string]any{},
			"talos": map[string]any{
				"imageFactoryURL": "https://factory.talos.dev",
				"schematicID":     "ce4c980550dd2ab1b17bbf2b08801c7eb59418eafe8f279833297925d67c7515",
				"version":         "v1.13.9",
			},
		},
	}

	if diags := resourceModel.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	version, _ := resourceModel.Talos.Attributes()[attrVersion].(types.String)
	if version.ValueString() != "v1.13.9" {
		t.Errorf("talos.version = %q, want the server's v1.13.9 so the drift is planned away", version.ValueString())
	}

	schematic, _ := resourceModel.Talos.Attributes()["schematic_id"].(types.String)
	if !schematic.IsNull() {
		t.Errorf("talos.schematic_id = %v, want null — the configuration never named it", schematic)
	}
}

// Nested objects are trimmed the same way, so a configured secret reference does
// not drag the inline config the server reports alongside it into state.
func TestKubernetesResourceFlatten_TrimsNestedObjects(t *testing.T) {
	t.Parallel()

	secretRef := types.ObjectValueMust(k8sOIDCSecretRefObjectType(), map[string]attr.Value{
		attrName: types.StringValue("tenant-authentication-config"),
	})

	resourceModel := kubernetesResourceModel{kubernetesModel: kubernetesModel{
		OIDC: types.ObjectValueMust(k8sOIDCObjectType(), map[string]attr.Value{
			"mode":  types.StringValue("CustomConfig"),
			"users": types.ListNull(types.ObjectType{AttrTypes: k8sOIDCUserObjectType()}),
			"custom_config": types.ObjectValueMust(k8sOIDCCustomConfigObjectType(), map[string]attr.Value{
				"config":     types.StringNull(),
				"secret_ref": secretRef,
			}),
		}),
	}}

	app := &client.Application{
		Name:      "cluster",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"nodeGroups": map[string]any{},
			"oidc": map[string]any{
				"mode":  "CustomConfig",
				"users": []any{},
				"customConfig": map[string]any{
					"config":    "",
					"secretRef": map[string]any{"name": "tenant-authentication-config"},
				},
			},
		},
	}

	if diags := resourceModel.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	users, _ := resourceModel.OIDC.Attributes()["users"].(types.List)
	if !users.IsNull() {
		t.Errorf("oidc.users = %v, want null — the server's empty list is not a configured value", users)
	}

	custom, _ := resourceModel.OIDC.Attributes()["custom_config"].(types.Object)

	config, _ := custom.Attributes()["config"].(types.String)
	if !config.IsNull() {
		t.Errorf("oidc.custom_config.config = %v, want null", config)
	}

	ref, _ := custom.Attributes()["secret_ref"].(types.Object)

	name, _ := ref.Attributes()[attrName].(types.String)
	if name.ValueString() != "tenant-authentication-config" {
		t.Errorf("oidc.custom_config.secret_ref.name = %q, want the configured Secret", name.ValueString())
	}
}

// storage_class carries no provider-side default, so an unset attribute leaves
// the key out and the platform supplies its own. Emitting a value the
// practitioner never chose would rewrite a field whose PersistentVolumeClaims
// can never follow it.
func TestKubernetesExpand_StorageClassOmittedWhenUnset(t *testing.T) {
	t.Parallel()

	model := fullKubernetesModel()
	model.StorageClass = types.StringNull()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if _, ok := got.Spec[specStorageClass]; ok {
		t.Errorf("storageClass present for an unset attribute, want omitted")
	}

	model.StorageClass = types.StringValue("local")

	got, diags = model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Spec[specStorageClass] != "local" {
		t.Errorf("storageClass = %v, want local", got.Spec[specStorageClass])
	}
}

// A node group with `roles = []` deliberately carries no role. Collapsing that
// to an absent key made the server return nothing, the state read back null, and
// the apply fail with an inconsistent-result error on a valid configuration.
func TestKubernetesExpand_NodeGroupRolesThreeStates(t *testing.T) {
	t.Parallel()

	group := func(roles types.List) types.Map {
		return types.MapValueMust(
			types.ObjectType{AttrTypes: k8sNodeGroupObjectType()},
			map[string]attr.Value{"md0": types.ObjectValueMust(k8sNodeGroupObjectType(), map[string]attr.Value{
				"disk_size":            types.StringValue("20Gi"),
				"instance_type":        types.StringValue("u1.medium"),
				"min_replicas":         types.Int64Value(0),
				"max_replicas":         types.Int64Value(1),
				"roles":                roles,
				"storage_class":        types.StringValue(""),
				"resources":            types.ObjectNull(resourcesObjectType()),
				"max_unhealthy":        types.StringNull(),
				"node_startup_timeout": types.StringNull(),
			})},
		)
	}

	tests := []struct {
		name    string
		roles   types.List
		present bool
		length  int
	}{
		{name: "null omits the key", roles: types.ListNull(types.StringType)},
		{
			name:    "empty writes an empty list",
			roles:   types.ListValueMust(types.StringType, []attr.Value{}),
			present: true,
		},
		{
			name:    "populated writes the roles",
			roles:   types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ingress-nginx")}),
			present: true,
			length:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := fullKubernetesModel()
			model.NodeGroups = group(tt.roles)

			got, diags := model.expand(context.Background())
			if diags.HasError() {
				t.Fatalf("expand diagnostics: %v", diags)
			}

			md0 := nestedSpec(t, got.Spec, "nodeGroups", "md0")

			raw, ok := md0["roles"]
			if ok != tt.present {
				t.Fatalf("roles present = %v, want %v", ok, tt.present)
			}

			if !tt.present {
				return
			}

			items, _ := raw.([]any)
			if len(items) != tt.length {
				t.Errorf("roles has %d entries, want %d", len(items), tt.length)
			}
		})
	}
}

func TestKubernetesFlatten_NodeGroupRolesKeepEmptyList(t *testing.T) {
	t.Parallel()

	groups, diags := flattenNodeGroups(map[string]any{"md0": map[string]any{"roles": []any{}}})
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	md0, _ := groups.Elements()["md0"].(types.Object)

	roles, _ := md0.Attributes()["roles"].(types.List)
	if roles.IsNull() {
		t.Errorf("roles = null for a stored empty list, want an empty list")
	}
}

// expand is guarded against the upstream json tags, but flatten's keys are
// hand-written on both sides — in the code and in the tests — so a matched typo
// would pass every other test in this file. Sending the full model through both
// directions makes the agreement transitive instead of eyeballed.
func TestKubernetesExpandFlatten_RoundTripsEveryBlock(t *testing.T) {
	t.Parallel()

	want := fullKubernetesModel()

	app, diags := want.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	var got kubernetesModel

	if diags := got.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	blocks := map[string][2]attr.Value{
		"talos":             {want.Talos, got.Talos},
		"node_health_check": {want.NodeHealthCheck, got.NodeHealthCheck},
		"oidc":              {want.OIDC, got.OIDC},
		"control_plane":     {want.ControlPlane, got.ControlPlane},
		"images":            {want.Images, got.Images},
		"node_groups":       {want.NodeGroups, got.NodeGroups},
	}

	for name, pair := range blocks {
		if !pair[0].Equal(pair[1]) {
			t.Errorf("%s did not survive expand→flatten:\n sent %v\n back %v", name, pair[0], pair[1])
		}
	}

	if got.StorageClass != want.StorageClass || got.Version != want.Version || got.Host != want.Host {
		t.Errorf("scalars did not survive: storage_class=%v version=%v host=%v",
			got.StorageClass, got.Version, got.Host)
	}
}

// `node_groups = {}` is what upstream defaults to and documents — the chart
// renders a single md0 that provisions nothing until the autoscaler reacts. The
// attribute is Required, so the plan holds an empty map and the read has to
// return one; collapsing it to null fails the apply on a configuration the
// platform explicitly supports, and the provider's own example uses it.
func TestKubernetesFlatten_EmptyNodeGroupsStayEmpty(t *testing.T) {
	t.Parallel()

	groups, diags := flattenNodeGroups(map[string]any{})
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if groups.IsNull() {
		t.Fatalf("node_groups = null for a cluster that declares none, want an empty map")
	}

	if len(groups.Elements()) != 0 {
		t.Errorf("node_groups has %d entries, want 0", len(groups.Elements()))
	}

	// An absent key is still absent: that is a server that did not report the
	// field at all, not a cluster with no groups.
	absent, diags := flattenNodeGroups(nil)
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !absent.IsNull() {
		t.Errorf("node_groups = %v for an absent key, want null", absent)
	}
}

// keepConfiguredAttributes builds a null for anything the server leaves out, so
// it has to work off the attribute's own type rather than the string/list/object
// shapes these blocks happen to use today.
func TestKeepConfiguredAttributes_NullsMatchTheAttributeType(t *testing.T) {
	t.Parallel()

	objectType := map[string]attr.Type{
		"text":   types.StringType,
		"count":  types.Int64Type,
		"toggle": types.BoolType,
	}

	configured := types.ObjectValueMust(objectType, map[string]attr.Value{
		"text":   types.StringValue("set"),
		"count":  types.Int64Value(3),
		"toggle": types.BoolValue(true),
	})

	// A server that reports none of the keys back.
	server := types.ObjectValueMust(objectType, map[string]attr.Value{
		"text":   types.StringNull(),
		"count":  types.Int64Null(),
		"toggle": types.BoolNull(),
	})

	empty := types.ObjectValueMust(map[string]attr.Type{}, map[string]attr.Value{})

	got, diags := keepConfiguredAttributes(context.Background(), configured, empty)
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}

	if !got.Equal(server) {
		t.Errorf("trimmed object = %v, want every attribute null at its own type", got)
	}
}
