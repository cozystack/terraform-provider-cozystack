package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
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

			namespace, name, ok := parseTenantImportID(tc.id)
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
