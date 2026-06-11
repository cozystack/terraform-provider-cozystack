package client_test

import (
	"context"
	"testing"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGet_PopulatesUID(t *testing.T) {
	t.Parallel()

	tenant := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.cozystack.io/v1alpha1",
		"kind":       "Tenant",
		"metadata": map[string]any{
			"namespace": "tenant-root",
			"name":      "root",
			"uid":       "b0d32ff4-1001-4c20-892f-a4837e017712",
		},
		"spec": map[string]any{},
	}}

	c := client.New(newFake(tenant))

	app, err := c.Get(context.Background(), client.TenantResource(), "tenant-root", "root")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if app.UID != "b0d32ff4-1001-4c20-892f-a4837e017712" {
		t.Errorf("UID = %q, want the seeded metadata.uid", app.UID)
	}
}
