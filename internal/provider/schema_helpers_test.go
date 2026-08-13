package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// storageClassReplaceExempt lists the storage class attributes this guard does
// not cover, by resource type and dotted attribute path.
//
// The kubernetes node-group class is deliberately mutable upstream: the field is
// optional and undefaulted there, so an immutability rule would block ever
// setting it on an existing node group.
var storageClassReplaceExempt = map[string]bool{
	"cozystack_kubernetes.node_groups.storage_class": true,
}

// TestStorageClassRequiresReplace checks that every storage_class attribute,
// nested ones included, plans a replacement when a configured value changes. A
// storage class is fixed when the volume is created: the aggregated apiserver
// accepts a change without acting on it, and nothing migrates the existing
// volume, so an in-place update would silently record a class the data does not
// live on.
func TestStorageClassRequiresReplace(t *testing.T) {
	t.Parallel()

	for typeName, found := range storageClassAttributes(t) {
		for path, attribute := range found {
			text, ok := attribute.(schema.StringAttribute)
			if !ok {
				t.Errorf("%s: %s is not a string attribute", typeName, path)

				continue
			}

			replaces := requiresReplace(t, text.PlanModifiers, types.StringValue("new-class"))

			// An exemption that no longer describes the schema is worse than
			// no exemption, because it keeps skipping an attribute that has
			// since joined the contract.
			if storageClassReplaceExempt[typeName+"."+path] {
				if replaces {
					t.Errorf("%s: %s requires replacement now; drop it from the exempt list", typeName, path)
				}

				continue
			}

			if !replaces {
				t.Errorf("%s: %s does not require replacement when it changes", typeName, path)
			}
		}
	}
}

// TestStorageClassKeepsUnconfiguredInstance is the other half of the contract.
// Attribute defaults are applied to the planned value whenever the
// configuration is null, and that happens before plan modifiers run, so an
// instance imported without the attribute — or one whose attribute was just
// deleted from the configuration — plans its default against the stored class.
// Replacing on that would destroy a database in response to an edit that
// removed nothing from the object.
func TestStorageClassKeepsUnconfiguredInstance(t *testing.T) {
	t.Parallel()

	for typeName, found := range storageClassAttributes(t) {
		for path, attribute := range found {
			text, ok := attribute.(schema.StringAttribute)
			if !ok {
				continue
			}

			if requiresReplace(t, text.PlanModifiers, types.StringNull()) {
				t.Errorf("%s: %s requires replacement when it is absent from the configuration", typeName, path)
			}
		}
	}
}

// storageClassAttributes collects every resource's storage_class attributes,
// keyed by resource type name and then by dotted attribute path.
func storageClassAttributes(t *testing.T) map[string]map[string]schema.Attribute {
	t.Helper()

	ctx := context.Background()
	out := map[string]map[string]schema.Attribute{}

	for _, factory := range New("test")().Resources(ctx) {
		res := factory()

		var metadata resource.MetadataResponse

		res.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "cozystack"}, &metadata)

		var schemaResp resource.SchemaResponse

		res.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("%s schema diagnostics: %v", metadata.TypeName, schemaResp.Diagnostics)
		}

		found := map[string]schema.Attribute{}
		collectStorageClassAttributes(schemaResp.Schema.Attributes, "", found)

		if len(found) > 0 {
			out[metadata.TypeName] = found
		}
	}

	return out
}

// collectStorageClassAttributes gathers every storage_class attribute in a
// schema, descending into nested blocks, keyed by its dotted path.
func collectStorageClassAttributes(attributes map[string]schema.Attribute, prefix string, found map[string]schema.Attribute) {
	for name, attribute := range attributes {
		path := prefix + name

		if name == attrStorageClass {
			found[path] = attribute
		}

		switch nested := attribute.(type) {
		case schema.SingleNestedAttribute:
			collectStorageClassAttributes(nested.Attributes, path+".", found)
		case schema.ListNestedAttribute:
			collectStorageClassAttributes(nested.NestedObject.Attributes, path+".", found)
		case schema.MapNestedAttribute:
			collectStorageClassAttributes(nested.NestedObject.Attributes, path+".", found)
		}
	}
}

// requiresReplace runs the modifiers against a changed value on an existing
// object and reports whether any of them asks for a replacement. A null config
// value models the attribute being absent from the configuration, where the
// planned value is the schema default rather than anything the practitioner
// asked for.
func requiresReplace(t *testing.T, modifiers []planmodifier.String, config types.String) bool {
	t.Helper()

	ctx := context.Background()
	existing := tftypes.NewValue(
		tftypes.Object{AttributeTypes: map[string]tftypes.Type{}},
		map[string]tftypes.Value{},
	)

	planned := config
	if config.IsNull() {
		planned = types.StringValue("")
	}

	req := planmodifier.StringRequest{
		State:       tfsdk.State{Raw: existing},
		Plan:        tfsdk.Plan{Raw: existing},
		StateValue:  types.StringValue("old-class"),
		PlanValue:   planned,
		ConfigValue: config,
	}

	for _, modifier := range modifiers {
		resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}

		modifier.PlanModifyString(ctx, req, resp)

		if resp.RequiresReplace {
			return true
		}
	}

	return false
}
