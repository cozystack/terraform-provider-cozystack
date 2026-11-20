package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKubernetesNodesWorkerClassSchema(t *testing.T) {
	t.Parallel()

	resource := kubernetesNodesSchema().Attributes
	class, ok := resource["worker_class_name"].(rschema.StringAttribute)
	if !ok {
		t.Fatal("worker_class_name must be a string attribute")
	}

	if !class.Optional || !class.Computed || class.Default != nil {
		t.Error("worker_class_name must accept a server-assigned class without a provider default")
	}

	keys, ok := resource["ssh_keys"].(rschema.ListAttribute)
	if !ok {
		t.Fatal("ssh_keys must be a list attribute")
	}

	if !keys.Optional || keys.Computed || keys.Default != nil || !keys.ElementType.Equal(types.StringType) {
		t.Error("ssh_keys must be optional strings without a default")
	}

	dataSource := kubernetesNodesDataSourceSchema().Attributes
	readClass, ok := dataSource["worker_class_name"].(dsschema.StringAttribute)
	if !ok || !readClass.Computed {
		t.Error("data source worker_class_name must be computed")
	}

	readKeys, ok := dataSource["ssh_keys"].(dsschema.ListAttribute)
	if !ok || !readKeys.Computed || !readKeys.ElementType.Equal(types.StringType) {
		t.Error("data source ssh_keys must be computed strings")
	}
}

func TestKubernetesNodesWorkerClassRoundTrip(t *testing.T) {
	t.Parallel()

	for _, class := range []types.String{types.StringNull(), types.StringValue("talos"), types.StringValue("kairos-example")} {
		t.Run(class.String(), func(t *testing.T) {
			t.Parallel()

			model := unsetKubernetesNodesModel()
			model.WorkerClassName = class
			app, diags := model.expand(context.Background())
			if diags.HasError() {
				t.Fatalf("expand: %v", diags)
			}

			value, present := app.Spec["workerClassName"]
			if present == class.IsNull() || (present && value != class.ValueString()) {
				t.Fatalf("workerClassName = %#v, present %v, want %s", value, present, class)
			}

			for _, id := range []types.String{types.StringNull(), types.StringValue("tenant-root/demo-md1")} {
				resource := kubernetesNodesResourceModel{kubernetesNodesModel: unsetKubernetesNodesModel()}
				resource.ID = id
				if diags := resource.flatten(app); diags.HasError() {
					t.Fatalf("flatten: %v", diags)
				}
				if !resource.WorkerClassName.Equal(class) {
					t.Fatalf("class after import/refresh = %s, want %s", resource.WorkerClassName, class)
				}
				updated, diags := resource.expand(context.Background())
				if diags.HasError() {
					t.Fatalf("update expand: %v", diags)
				}
				if !reflect.DeepEqual(updated.Spec["workerClassName"], value) {
					t.Fatalf("update class = %#v, want %#v", updated.Spec["workerClassName"], value)
				}
			}
		})
	}
}

func TestKubernetesNodesWorkerClassServerAssigned(t *testing.T) {
	t.Parallel()

	model := kubernetesNodesResourceModel{kubernetesNodesModel: unsetKubernetesNodesModel()}
	model.ID = types.StringUnknown()
	model.WorkerClassName = types.StringUnknown()
	app, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
	if _, present := app.Spec["workerClassName"]; present {
		t.Fatal("unknown class must not be sent on CREATE")
	}

	app.Spec["workerClassName"] = "talos"
	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}
	if !model.WorkerClassName.Equal(types.StringValue("talos")) {
		t.Fatalf("server-assigned class = %s, want talos", model.WorkerClassName)
	}
}

func TestKubernetesNodesSSHKeysRoundTrip(t *testing.T) {
	t.Parallel()

	for _, keys := range []types.List{
		types.ListNull(types.StringType),
		types.ListValueMust(types.StringType, []attr.Value{}),
		types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ssh-ed25519 EXAMPLE")}),
	} {
		t.Run(keys.String(), func(t *testing.T) {
			t.Parallel()

			model := unsetKubernetesNodesModel()
			model.SSHKeys = keys
			app, diags := model.expand(context.Background())
			if diags.HasError() {
				t.Fatalf("expand: %v", diags)
			}
			if _, present := app.Spec["sshKeys"]; present == keys.IsNull() {
				t.Fatalf("sshKeys presence = %v for %s", present, keys)
			}

			resource := kubernetesNodesResourceModel{kubernetesNodesModel: model}
			if diags := resource.flatten(app); diags.HasError() {
				t.Fatalf("resource flatten: %v", diags)
			}
			if !resource.SSHKeys.Equal(keys) {
				t.Fatalf("resource ssh keys = %s, want %s", resource.SSHKeys, keys)
			}

			var dataSource kubernetesNodesModel
			if diags := dataSource.flatten(&client.Application{Spec: app.Spec}); diags.HasError() {
				t.Fatalf("data source flatten: %v", diags)
			}
			if !dataSource.SSHKeys.Equal(keys) {
				t.Fatalf("data source ssh keys = %s, want %s", dataSource.SSHKeys, keys)
			}
		})
	}
}

func TestKubernetesNodesWorkerClassRejectsEmptyConfig(t *testing.T) {
	t.Parallel()
	attribute := kubernetesNodesSchema().Attributes["worker_class_name"].(rschema.StringAttribute)
	for _, value := range []types.String{types.StringValue(""), types.StringValue("talos"), types.StringNull(), types.StringUnknown()} {
		failed := false
		for _, check := range attribute.Validators {
			response := &validator.StringResponse{}
			check.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("worker_class_name"), ConfigValue: value}, response)
			failed = failed || response.Diagnostics.HasError()
		}
		if failed != value.Equal(types.StringValue("")) {
			t.Errorf("class %s rejected = %v", value, failed)
		}
	}
}

func TestKubernetesNodesWorkerClassLegacyEmpty(t *testing.T) {
	t.Parallel()
	model := kubernetesNodesResourceModel{kubernetesNodesModel: unsetKubernetesNodesModel()}
	app := serverResolvedApplication()
	app.Spec["workerClassName"] = ""
	app.Spec["sshKeys"] = []any{}
	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}
	if !model.WorkerClassName.IsNull() || !model.SSHKeys.IsNull() {
		t.Fatalf("legacy state class=%s keys=%s", model.WorkerClassName, model.SSHKeys)
	}
	updated, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
	if _, present := updated.Spec["workerClassName"]; present {
		t.Fatal("legacy update must omit workerClassName")
	}
	if _, present := updated.Spec["sshKeys"]; present {
		t.Fatal("legacy update must omit sshKeys")
	}
}

func TestKubernetesNodesWorkerImportGuidance(t *testing.T) {
	t.Parallel()

	_, importGuidance, found := strings.Cut(kubernetesNodesSchema().MarkdownDescription, "Import captures")
	if !found || !strings.Contains(importGuidance, "`ssh_keys`") {
		t.Fatal("import guidance must warn users to configure imported ssh_keys before applying")
	}
}
