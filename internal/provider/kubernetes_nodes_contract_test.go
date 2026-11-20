package provider

import (
	"context"
	"testing"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKubernetesNodesTaintValidators(t *testing.T) {
	t.Parallel()

	taints := kubernetesNodesSchema().Attributes["taints"].(rschema.ListNestedAttribute)
	cases := []struct {
		name      string
		attribute string
		value     types.String
		rejected  bool
	}{
		{"empty key", "key", types.StringValue(""), true},
		{"nonempty key", "key", types.StringValue("example.com/workload"), false},
		{"null key", "key", types.StringNull(), false},
		{"unknown key", "key", types.StringUnknown(), false},
		{"empty effect", "effect", types.StringValue(""), true},
		{"invalid effect", "effect", types.StringValue("invalid"), true},
		{"lowercase effect", "effect", types.StringValue("noschedule"), true},
		{"no schedule", "effect", types.StringValue("NoSchedule"), false},
		{"prefer no schedule", "effect", types.StringValue("PreferNoSchedule"), false},
		{"no execute", "effect", types.StringValue("NoExecute"), false},
		{"null effect", "effect", types.StringNull(), false},
		{"unknown effect", "effect", types.StringUnknown(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			attribute := taints.NestedObject.Attributes[tc.attribute].(rschema.StringAttribute)
			rejected := false
			for _, check := range attribute.Validators {
				response := &validator.StringResponse{}
				check.ValidateString(context.Background(), validator.StringRequest{
					Path:        path.Root("taints").AtListIndex(0).AtName(tc.attribute),
					ConfigValue: tc.value,
				}, response)
				rejected = rejected || response.Diagnostics.HasError()
			}
			if rejected != tc.rejected {
				t.Errorf("%s %s rejected = %v, want %v", tc.attribute, tc.value, rejected, tc.rejected)
			}
		})
	}
}

func TestKubernetesNodesLabelsAndTaintsSchemas(t *testing.T) {
	t.Parallel()

	resource := kubernetesNodesSchema().Attributes
	labels, ok := resource["labels"].(rschema.MapAttribute)
	if !ok || !labels.Optional || labels.Computed || labels.Default != nil || !labels.ElementType.Equal(types.StringType) {
		t.Error("labels must be an optional string map without a provider default")
	}

	taints, ok := resource["taints"].(rschema.ListNestedAttribute)
	if !ok || !taints.Optional || taints.Computed {
		t.Error("taints must be an optional nested list without a provider default")
	}

	dataSource := kubernetesNodesDataSourceSchema().Attributes
	readLabels, ok := dataSource["labels"].(dsschema.MapAttribute)
	if !ok || !readLabels.Computed || !readLabels.ElementType.Equal(types.StringType) {
		t.Error("data source labels must be a computed string map")
	}

	readTaints, ok := dataSource["taints"].(dsschema.ListNestedAttribute)
	if !ok || !readTaints.Computed {
		t.Error("data source taints must be a computed nested list")
	}
}
