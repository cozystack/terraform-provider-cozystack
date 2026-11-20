package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKubernetesNodesStartupDefaultMatchesChart(t *testing.T) {
	t.Parallel()

	attribute := kubernetesNodesSchema().Attributes["node_startup_timeout"].(rschema.StringAttribute)
	var response defaults.StringResponse
	attribute.Default.DefaultString(context.Background(), defaults.StringRequest{Path: path.Root("node_startup_timeout")}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("startup default diagnostics: %v", response.Diagnostics)
	}

	if got := response.PlanValue.ValueString(); got != "20m" {
		t.Errorf("startup default = %q, want chart default 20m", got)
	}
}

func TestKubernetesNodesExplicitStartupTimeoutIsPreserved(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"10m", "45m"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			model := unsetKubernetesNodesModel()
			model.NodeStartupTimeout = types.StringValue(value)
			app, diags := model.expand(context.Background())
			if diags.HasError() {
				t.Fatalf("expand: %v", diags)
			}
			if got := app.Spec["nodeStartupTimeout"]; got != value {
				t.Fatalf("explicit timeout = %v, want %s", got, value)
			}
			if diags := model.flatten(app); diags.HasError() {
				t.Fatalf("flatten: %v", diags)
			}
			if got := model.NodeStartupTimeout.ValueString(); got != value {
				t.Fatalf("read timeout = %s, want %s", got, value)
			}
		})
	}
}
