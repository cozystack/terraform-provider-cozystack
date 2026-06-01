package client_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newFake(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "apps.cozystack.io", Version: "v1alpha1", Resource: "tenants"}: "TenantList",
	}

	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
}

func TestCreateTenant_RoundTrips(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())

	in := client.Tenant{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"etcd": true,
			"host": "dev.example.test",
		},
	}

	out, err := c.CreateTenant(context.Background(), &in)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	if out.Name != "dev" || out.Namespace != "tenant-root" {
		t.Errorf("identity = %s/%s, want tenant-root/dev", out.Namespace, out.Name)
	}
	if out.Spec["etcd"] != true {
		t.Errorf("spec.etcd = %v, want true", out.Spec["etcd"])
	}
	if out.Spec["host"] != "dev.example.test" {
		t.Errorf("spec.host = %v, want dev.example.test", out.Spec["host"])
	}
}

func TestCreateThenGetTenant(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())
	ctx := context.Background()

	if _, err := c.CreateTenant(ctx, &client.Tenant{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"monitoring": true},
	}); err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	got, err := c.GetTenant(ctx, "tenant-root", "dev")
	if err != nil {
		t.Fatalf("GetTenant() error = %v", err)
	}

	if got.Spec["monitoring"] != true {
		t.Errorf("spec.monitoring = %v, want true", got.Spec["monitoring"])
	}
}

func TestGetTenant_NotFound(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())

	_, err := c.GetTenant(context.Background(), "tenant-root", "missing")
	if !client.IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false, want true", err)
	}
}

func TestGetTenant_ExtractsStatus(t *testing.T) {
	t.Parallel()

	seed := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.cozystack.io/v1alpha1",
		"kind":       "Tenant",
		"metadata": map[string]any{
			"name":      "root",
			"namespace": "tenant-root",
		},
		"spec": map[string]any{"etcd": true},
		"status": map[string]any{
			"namespace": "tenant-root",
			"version":   "1.2.3",
			"conditions": []any{
				map[string]any{"type": "Released", "status": "True"},
				map[string]any{"type": "Ready", "status": "True"},
			},
		},
	}}

	c := client.New(newFake(seed))

	got, err := c.GetTenant(context.Background(), "tenant-root", "root")
	if err != nil {
		t.Fatalf("GetTenant() error = %v", err)
	}

	if !got.Status.Ready {
		t.Errorf("Status.Ready = false, want true")
	}
	if got.Status.Namespace != "tenant-root" {
		t.Errorf("Status.Namespace = %q, want tenant-root", got.Status.Namespace)
	}
	if got.Status.Version != "1.2.3" {
		t.Errorf("Status.Version = %q, want 1.2.3", got.Status.Version)
	}
}

func TestGetTenant_NotReadyWhenConditionFalse(t *testing.T) {
	t.Parallel()

	seed := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.cozystack.io/v1alpha1",
		"kind":       "Tenant",
		"metadata":   map[string]any{"name": "root", "namespace": "tenant-root"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False"},
			},
		},
	}}

	c := client.New(newFake(seed))

	got, err := c.GetTenant(context.Background(), "tenant-root", "root")
	if err != nil {
		t.Fatalf("GetTenant() error = %v", err)
	}

	if got.Status.Ready {
		t.Errorf("Status.Ready = true, want false")
	}
}

func TestUpdateTenant_ChangesSpec(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())
	ctx := context.Background()

	if _, err := c.CreateTenant(ctx, &client.Tenant{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"monitoring": false},
	}); err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	if _, err := c.UpdateTenant(ctx, &client.Tenant{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"monitoring": true, "ingress": true},
	}); err != nil {
		t.Fatalf("UpdateTenant() error = %v", err)
	}

	got, err := c.GetTenant(ctx, "tenant-root", "dev")
	if err != nil {
		t.Fatalf("GetTenant() error = %v", err)
	}

	if got.Spec["monitoring"] != true {
		t.Errorf("spec.monitoring = %v, want true", got.Spec["monitoring"])
	}
	if got.Spec["ingress"] != true {
		t.Errorf("spec.ingress = %v, want true", got.Spec["ingress"])
	}
}

func TestUpdateTenant_RetriesOnConflict(t *testing.T) {
	t.Parallel()

	fake := newFake()
	c := client.New(fake)
	ctx := context.Background()

	if _, err := c.CreateTenant(ctx, &client.Tenant{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"etcd": false},
	}); err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	var updates int

	fake.PrependReactor("update", "tenants", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		updates++
		if updates == 1 {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: "apps.cozystack.io", Resource: "tenants"},
				"dev", errors.New("the object has been modified"),
			)
		}

		return false, nil, nil
	})

	if _, err := c.UpdateTenant(ctx, &client.Tenant{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"etcd": true},
	}); err != nil {
		t.Fatalf("UpdateTenant() error = %v, want nil after one conflict", err)
	}

	if updates < 2 {
		t.Errorf("update attempts = %d, want at least 2 (one conflict + one success)", updates)
	}
}

func TestDeleteTenant(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())
	ctx := context.Background()

	if _, err := c.CreateTenant(ctx, &client.Tenant{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{},
	}); err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	if err := c.DeleteTenant(ctx, "tenant-root", "dev"); err != nil {
		t.Fatalf("DeleteTenant() error = %v", err)
	}

	if _, err := c.GetTenant(ctx, "tenant-root", "dev"); !client.IsNotFound(err) {
		t.Errorf("after delete, IsNotFound = false (err=%v), want true", err)
	}
}

func TestDeleteTenant_AlreadyGoneIsSuccess(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())

	if err := c.DeleteTenant(context.Background(), "tenant-root", "missing"); err != nil {
		t.Errorf("DeleteTenant(missing) error = %v, want nil", err)
	}
}
