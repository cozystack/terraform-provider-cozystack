package provider_test

import (
	"context"
	"testing"

	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/lexfrei/terraform-provider-cozystack/internal/provider"
)

func TestProviderMetadata(t *testing.T) {
	t.Parallel()

	p := provider.New("1.2.3")()

	var resp fwprovider.MetadataResponse

	p.Metadata(context.Background(), fwprovider.MetadataRequest{}, &resp)

	if resp.TypeName != "cozystack" {
		t.Errorf("TypeName = %q, want cozystack", resp.TypeName)
	}
	if resp.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", resp.Version)
	}
}

func TestProviderSchema(t *testing.T) {
	t.Parallel()

	p := provider.New("test")()

	var resp fwprovider.SchemaResponse

	p.Schema(context.Background(), fwprovider.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema produced diagnostics: %v", resp.Diagnostics)
	}

	want := []string{
		"host",
		"token",
		"cluster_ca_certificate",
		"insecure",
		"config_path",
		"config_context",
		"in_cluster",
	}

	for _, name := range want {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("provider schema missing attribute %q", name)
		}
	}

	token, ok := resp.Schema.Attributes["token"]
	if !ok || !token.IsSensitive() {
		t.Errorf("token attribute should be marked sensitive")
	}
}
