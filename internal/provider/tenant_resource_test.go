package provider

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestTenantResourceMetadata(t *testing.T) {
	t.Parallel()

	res := NewTenantResource()

	var resp resource.MetadataResponse

	res.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "cozystack"}, &resp)

	if resp.TypeName != "cozystack_tenant" {
		t.Errorf("TypeName = %q, want cozystack_tenant", resp.TypeName)
	}
}

func TestTenantSchemaAttributes(t *testing.T) {
	t.Parallel()

	attributes := tenantSchema().Attributes

	required := []string{"name", "namespace"}
	for _, name := range required {
		attribute, ok := attributes[name]
		if !ok {
			t.Errorf("schema missing required attribute %q", name)

			continue
		}

		if !attribute.IsRequired() {
			t.Errorf("attribute %q should be required", name)
		}
	}

	computed := []string{"id", "status_namespace", "ready", "version"}
	for _, name := range computed {
		attribute, ok := attributes[name]
		if !ok {
			t.Errorf("schema missing computed attribute %q", name)

			continue
		}

		if !attribute.IsComputed() {
			t.Errorf("attribute %q should be computed", name)
		}
	}

	optionalComputed := []string{"host", "etcd", "monitoring", "ingress", "seaweedfs", "scheduling_class", "resource_quotas"}
	for _, name := range optionalComputed {
		attribute, ok := attributes[name]
		if !ok {
			t.Errorf("schema missing attribute %q", name)

			continue
		}

		if !attribute.IsOptional() || !attribute.IsComputed() {
			t.Errorf("attribute %q should be optional and computed", name)
		}
	}
}

func TestParseTenantImportID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		id        string
		namespace string
		tenant    string
		ok        bool
	}{
		{name: "valid", id: "tenant-root/dev", namespace: "tenant-root", tenant: "dev", ok: true},
		{name: "no separator", id: "dev", ok: false},
		{name: "empty namespace", id: "/dev", ok: false},
		{name: "empty name", id: "tenant-root/", ok: false},
		{name: "empty", id: "", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			namespace, name, ok := parseImportID(tc.id)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}

			if !tc.ok {
				return
			}

			if namespace != tc.namespace || name != tc.tenant {
				t.Errorf("got %s/%s, want %s/%s", namespace, name, tc.namespace, tc.tenant)
			}
		})
	}
}

func TestParseWaitTimeout(t *testing.T) {
	t.Parallel()

	t.Run("valid duration", func(t *testing.T) {
		t.Parallel()

		got, diags := parseWaitTimeout(types.StringValue("10m"))
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		if got != 10*time.Minute {
			t.Errorf("duration = %s, want 10m", got)
		}
	})

	t.Run("empty falls back to default", func(t *testing.T) {
		t.Parallel()

		got, diags := parseWaitTimeout(types.StringValue(""))
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		if got != defaultWaitTimeout {
			t.Errorf("duration = %s, want %s", got, defaultWaitTimeout)
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		t.Parallel()

		_, diags := parseWaitTimeout(types.StringValue("not-a-duration"))
		if !diags.HasError() {
			t.Error("expected diagnostics for an invalid duration")
		}
	})
}

// TestTenantResourceModelRoundTrip verifies that the resource model — including
// the embedded tenantModel and the wait attributes — binds to and from the
// schema-typed state without loss.
func TestTenantResourceModelRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resourceSchema := tenantSchema()
	state := tfsdk.State{
		Schema: resourceSchema,
		Raw:    tftypes.NewValue(resourceSchema.Type().TerraformType(ctx), nil),
	}

	in := tenantResourceModel{
		tenantModel: tenantModel{
			ID:              types.StringValue("tenant-root/dev"),
			Name:            types.StringValue("dev"),
			Namespace:       types.StringValue("tenant-root"),
			Host:            types.StringValue(""),
			Etcd:            types.BoolValue(true),
			Monitoring:      types.BoolValue(false),
			Ingress:         types.BoolValue(false),
			Seaweedfs:       types.BoolValue(false),
			SchedulingClass: types.StringValue(""),
			ResourceQuotas:  types.MapValueMust(types.StringType, map[string]attr.Value{}),
			StatusNamespace: types.StringValue("tenant-dev"),
			Ready:           types.BoolValue(true),
			Version:         types.StringValue("1.4.2"),
		},
		WaitForReady: types.BoolValue(true),
		WaitTimeout:  types.StringValue("5m"),
	}

	if diags := state.Set(ctx, &in); diags.HasError() {
		t.Fatalf("state.Set: %v", diags)
	}

	var out tenantResourceModel

	if diags := state.Get(ctx, &out); diags.HasError() {
		t.Fatalf("state.Get: %v", diags)
	}

	if out.Name.ValueString() != "dev" || out.Namespace.ValueString() != "tenant-root" {
		t.Errorf("identity round-trip = %s/%s", out.Namespace.ValueString(), out.Name.ValueString())
	}
	if !out.Etcd.ValueBool() {
		t.Errorf("etcd round-trip lost")
	}
	if !out.WaitForReady.ValueBool() || out.WaitTimeout.ValueString() != "5m" {
		t.Errorf("wait attributes round-trip lost: %v/%q", out.WaitForReady.ValueBool(), out.WaitTimeout.ValueString())
	}
}
