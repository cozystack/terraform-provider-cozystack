package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func TestTenantDataSourceMetadata(t *testing.T) {
	t.Parallel()

	ds := NewTenantDataSource()

	var resp datasource.MetadataResponse

	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "cozystack"}, &resp)

	if resp.TypeName != "cozystack_tenant" {
		t.Errorf("TypeName = %q, want cozystack_tenant", resp.TypeName)
	}
}

func TestTenantDataSourceSchema(t *testing.T) {
	t.Parallel()

	ds := NewTenantDataSource()

	var resp datasource.SchemaResponse

	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema produced diagnostics: %v", resp.Diagnostics)
	}

	attributes := resp.Schema.Attributes

	for _, name := range []string{"name", "namespace"} {
		attribute, ok := attributes[name]
		if !ok || !attribute.IsRequired() {
			t.Errorf("attribute %q should be required", name)
		}
	}

	computed := []string{
		"id", "host", "etcd", "monitoring", "ingress", "seaweedfs",
		"scheduling_class", "resource_quotas", "status_namespace", "ready", "version",
	}

	for _, name := range computed {
		attribute, ok := attributes[name]
		if !ok || !attribute.IsComputed() {
			t.Errorf("attribute %q should be computed", name)
		}
	}
}
