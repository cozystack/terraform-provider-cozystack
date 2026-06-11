package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/kubernetes"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
	}
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
