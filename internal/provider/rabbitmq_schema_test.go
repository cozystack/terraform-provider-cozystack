package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
)

func TestRabbitmqSchemaResourcesPresetDefault(t *testing.T) {
	t.Parallel()

	attribute, ok := rabbitmqSchema().Attributes[attrResourcesPreset].(rschema.StringAttribute)
	if !ok {
		t.Fatalf("%s is not a string attribute", attrResourcesPreset)
	}
	if attribute.Default == nil {
		t.Fatalf("%s has no default", attrResourcesPreset)
	}

	var response defaults.StringResponse
	attribute.Default.DefaultString(context.Background(), defaults.StringRequest{Path: path.Root(attrResourcesPreset)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("%s default diagnostics: %v", attrResourcesPreset, response.Diagnostics)
	}
	if got, want := response.PlanValue.ValueString(), "u1.nano"; got != want {
		t.Errorf("%s default = %q, want %q", attrResourcesPreset, got, want)
	}
}
