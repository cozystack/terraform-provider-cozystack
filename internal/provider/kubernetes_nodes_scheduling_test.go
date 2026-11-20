package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func schedulingTaintType() map[string]attr.Type {
	return map[string]attr.Type{"key": types.StringType, "value": types.StringType, "effect": types.StringType}
}

func schedulingValues() (types.Map, types.List) {
	labels := types.MapValueMust(types.StringType, map[string]attr.Value{"example.com/pool": types.StringValue("batch")})
	taints := types.ListValueMust(types.ObjectType{AttrTypes: schedulingTaintType()}, []attr.Value{
		types.ObjectValueMust(schedulingTaintType(), map[string]attr.Value{
			"key": types.StringValue("example.com/dedicated"), "value": types.StringValue("batch"), "effect": types.StringValue("NoSchedule"),
		}),
	})

	return labels, taints
}

func TestKubernetesNodesSchedulingPresenceRoundTrip(t *testing.T) {
	t.Parallel()

	populatedLabels, populatedTaints := schedulingValues()
	objectType := types.ObjectType{AttrTypes: schedulingTaintType()}
	for _, tc := range []struct {
		name   string
		labels types.Map
		taints types.List
	}{
		{"absent", types.MapNull(types.StringType), types.ListNull(objectType)},
		{"empty", types.MapValueMust(types.StringType, map[string]attr.Value{}), types.ListValueMust(objectType, []attr.Value{})},
		{"populated", populatedLabels, populatedTaints},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			model := unsetKubernetesNodesModel()
			model.Labels, model.Taints = tc.labels, tc.taints
			app, diags := model.expand(context.Background())
			if diags.HasError() {
				t.Fatalf("expand: %v", diags)
			}
			for _, key := range []string{"labels", "taints"} {
				_, present := app.Spec[key]
				if present == tc.labels.IsNull() {
					t.Fatalf("%s presence changed for %s", key, tc.name)
				}
			}
			resource := kubernetesNodesResourceModel{kubernetesNodesModel: model}
			if diags := resource.flatten(app); diags.HasError() {
				t.Fatalf("resource read: %v", diags)
			}
			if !resource.Labels.Equal(tc.labels) || !resource.Taints.Equal(tc.taints) {
				t.Fatal("resource changed omitted, empty or configured scheduling values")
			}
			var dataSource kubernetesNodesModel
			if diags := dataSource.flatten(app); diags.HasError() {
				t.Fatalf("data source read: %v", diags)
			}
			if !dataSource.Labels.Equal(tc.labels) || !dataSource.Taints.Equal(tc.taints) {
				t.Fatal("data source lost scheduling values")
			}
		})
	}
}

func TestKubernetesNodesUnknownSchedulingDoesNotBecomeEmpty(t *testing.T) {
	t.Parallel()

	model := unsetKubernetesNodesModel()
	model.Labels = types.MapUnknown(types.StringType)
	model.Taints = types.ListUnknown(types.ObjectType{AttrTypes: schedulingTaintType()})
	app, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
	for _, key := range []string{"labels", "taints"} {
		if _, present := app.Spec[key]; present {
			t.Fatalf("unknown %s was submitted", key)
		}
	}
}

func TestKubernetesNodesSchedulingSurvivesCRUDImportRefreshUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api := client.New(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()))
	res := client.KubernetesNodesResource()
	labels, taints := schedulingValues()
	model := unsetKubernetesNodesModel()
	model.Labels, model.Taints = labels, taints
	app, diags := model.expand(ctx)
	if diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
	created, err := api.Create(ctx, res, app)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, present := created.Spec["labels"]; !present {
		t.Fatal("CREATE lost configured labels")
	}
	read, err := api.Get(ctx, res, app.Namespace, app.Name)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	var imported kubernetesNodesResourceModel
	if diags := imported.flatten(&read); diags.HasError() {
		t.Fatalf("import read: %v", diags)
	}
	if !imported.Labels.Equal(labels) || !imported.Taints.Equal(taints) {
		t.Fatal("import failed to capture the scheduling settings")
	}
	imported.MinReplicas = types.Int64Value(2)
	update, diags := imported.expand(ctx)
	if diags.HasError() {
		t.Fatalf("update expand: %v", diags)
	}
	updated, err := api.Update(ctx, res, update)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	for _, key := range []string{"labels", "taints"} {
		if !reflect.DeepEqual(read.Spec[key], updated.Spec[key]) {
			t.Fatalf("unrelated update changed %s", key)
		}
	}
	if diags := imported.flatten(&updated); diags.HasError() {
		t.Fatalf("refresh after update: %v", diags)
	}
	if !imported.Labels.Equal(labels) || !imported.Taints.Equal(taints) {
		t.Fatal("refresh changed configured scheduling values")
	}
	imported.Labels = types.MapValueMust(types.StringType, map[string]attr.Value{})
	imported.Taints = types.ListValueMust(types.ObjectType{AttrTypes: schedulingTaintType()}, []attr.Value{})
	clearApp, diags := imported.expand(ctx)
	if diags.HasError() {
		t.Fatalf("clear expand: %v", diags)
	}
	cleared, err := api.Update(ctx, res, clearApp)
	if err != nil {
		t.Fatalf("clear update: %v", err)
	}
	if len(cleared.Spec["labels"].(map[string]any)) != 0 || len(cleared.Spec["taints"].([]any)) != 0 {
		t.Fatal("explicit empty values did not clear scheduling settings")
	}
}

func TestKubernetesNodesTaintValuePresence(t *testing.T) {
	t.Parallel()

	for _, value := range []types.String{types.StringNull(), types.StringValue(""), types.StringValue("batch")} {
		t.Run(value.String(), func(t *testing.T) {
			t.Parallel()

			model := unsetKubernetesNodesModel()
			model.Taints = types.ListValueMust(types.ObjectType{AttrTypes: schedulingTaintType()}, []attr.Value{
				types.ObjectValueMust(schedulingTaintType(), map[string]attr.Value{
					"key": types.StringValue("example.com/dedicated"), "value": value, "effect": types.StringValue("NoSchedule"),
				}),
			})
			app, diags := model.expand(context.Background())
			if diags.HasError() {
				t.Fatalf("expand: %v", diags)
			}
			entry := app.Spec["taints"].([]any)[0].(map[string]any)
			if _, present := entry["value"]; present == value.IsNull() {
				t.Fatal("taint value omission became an explicit empty value")
			}
			resource := kubernetesNodesResourceModel{kubernetesNodesModel: model}
			if diags := resource.flatten(app); diags.HasError() {
				t.Fatalf("flatten: %v", diags)
			}
			if !resource.Taints.Equal(model.Taints) {
				t.Fatal("taint value presence was not preserved")
			}
		})
	}
}

func TestKubernetesNodesSchedulingServerDefaultsDoNotPinConfiguration(t *testing.T) {
	t.Parallel()

	app := serverResolvedApplication()
	app.Spec["labels"] = map[string]any{}
	app.Spec["taints"] = []any{}
	resource := kubernetesNodesResourceModel{kubernetesNodesModel: unsetKubernetesNodesModel()}
	if diags := resource.flatten(app); diags.HasError() {
		t.Fatalf("resource read: %v", diags)
	}
	if !resource.Labels.IsNull() || !resource.Taints.IsNull() {
		t.Fatal("server defaults pinned previously omitted scheduling configuration")
	}
	var dataSource kubernetesNodesModel
	if diags := dataSource.flatten(app); diags.HasError() {
		t.Fatalf("data source read: %v", diags)
	}
	if dataSource.Labels.IsNull() || dataSource.Taints.IsNull() {
		t.Fatal("data source concealed present server values")
	}
}
