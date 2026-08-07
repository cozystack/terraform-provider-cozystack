package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/kubernetes"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
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

func fullKubernetesModel() kubernetesModel {
	group := types.ObjectValueMust(k8sNodeGroupObjectType(), map[string]attr.Value{
		"disk_size":     types.StringValue("20Gi"),
		"instance_type": types.StringValue("u1.medium"),
		"min_replicas":  types.Int64Value(0),
		"max_replicas":  types.Int64Value(3),
		"roles":         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ingress-nginx")}),
		"storage_class": types.StringValue(""),
		"resources":     types.ObjectNull(resourcesObjectType()),
	})

	return kubernetesModel{
		Name:         types.StringValue("cluster"),
		Namespace:    types.StringValue("tenant-root"),
		StorageClass: types.StringValue("replicated"),
		Version:      types.StringValue("v1.35"),
		Host:         types.StringValue("cluster.example.com"),
		NodeGroups:   types.MapValueMust(types.ObjectType{AttrTypes: k8sNodeGroupObjectType()}, map[string]attr.Value{"md0": group}),
		Talos:        fullTalosObject(),
	}
}

// nestedSpecKeys walks into a nested block of the emitted spec and returns its
// keys, so the coverage guard can be pointed at a struct below the top level.
func nestedSpecKeys(t *testing.T, spec map[string]any, path ...string) map[string]bool {
	t.Helper()

	current := spec

	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			t.Fatalf("spec block %q missing from the emitted spec", strings.Join(path, "."))
		}

		current = next
	}

	keys := make(map[string]bool, len(current))
	for key := range current {
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

	assertSpecCoverage(t, emitted, kubernetes.ConfigSpec{}, "addons", "controlPlane", "images")
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
