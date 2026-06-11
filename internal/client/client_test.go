package client_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
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

func TestCreate_RoundTrips(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())

	in := client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"etcd": true,
			"host": "dev.example.test",
		},
	}

	out, err := c.Create(context.Background(), client.TenantResource(), &in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
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

func TestCreateThenGet(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())
	ctx := context.Background()
	res := client.TenantResource()

	if _, err := c.Create(ctx, res, &client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"monitoring": true},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := c.Get(ctx, res, "tenant-root", "dev")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Spec["monitoring"] != true {
		t.Errorf("spec.monitoring = %v, want true", got.Spec["monitoring"])
	}
}

func TestGet_NotFound(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())

	_, err := c.Get(context.Background(), client.TenantResource(), "tenant-root", "missing")
	if !client.IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false, want true", err)
	}
}

func TestGet_ExtractsStatus(t *testing.T) {
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

	got, err := c.Get(context.Background(), client.TenantResource(), "tenant-root", "root")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !got.Status.Ready {
		t.Errorf("Status.Ready = false, want true")
	}
	if got.Status.Version != "1.2.3" {
		t.Errorf("Status.Version = %q, want 1.2.3", got.Status.Version)
	}
	if got.Status.Raw["namespace"] != "tenant-root" {
		t.Errorf("Status.Raw[namespace] = %v, want tenant-root", got.Status.Raw["namespace"])
	}
}

func TestGet_NotReadyWhenConditionFalse(t *testing.T) {
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

	got, err := c.Get(context.Background(), client.TenantResource(), "tenant-root", "root")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Status.Ready {
		t.Errorf("Status.Ready = true, want false")
	}
}

func TestUpdate_ChangesSpec(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())
	ctx := context.Background()
	res := client.TenantResource()

	if _, err := c.Create(ctx, res, &client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"monitoring": false},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := c.Update(ctx, res, &client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"monitoring": true, "ingress": true},
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := c.Get(ctx, res, "tenant-root", "dev")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Spec["monitoring"] != true {
		t.Errorf("spec.monitoring = %v, want true", got.Spec["monitoring"])
	}
	if got.Spec["ingress"] != true {
		t.Errorf("spec.ingress = %v, want true", got.Spec["ingress"])
	}
}

func TestUpdate_RetriesOnConflict(t *testing.T) {
	t.Parallel()

	fake := newFake()
	c := client.New(fake)
	ctx := context.Background()
	res := client.TenantResource()

	if _, err := c.Create(ctx, res, &client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"etcd": false},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
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

	if _, err := c.Update(ctx, res, &client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{"etcd": true},
	}); err != nil {
		t.Fatalf("Update() error = %v, want nil after one conflict", err)
	}

	if updates < 2 {
		t.Errorf("update attempts = %d, want at least 2 (one conflict + one success)", updates)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())
	ctx := context.Background()
	res := client.TenantResource()

	if _, err := c.Create(ctx, res, &client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := c.Delete(ctx, res, "tenant-root", "dev"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := c.Get(ctx, res, "tenant-root", "dev"); !client.IsNotFound(err) {
		t.Errorf("after delete, IsNotFound = false (err=%v), want true", err)
	}
}

func TestDelete_AlreadyGoneIsSuccess(t *testing.T) {
	t.Parallel()

	c := client.New(newFake())

	if err := c.Delete(context.Background(), client.TenantResource(), "tenant-root", "missing"); err != nil {
		t.Errorf("Delete(missing) error = %v, want nil", err)
	}
}

func TestWaitForReady_AlreadyReady(t *testing.T) {
	t.Parallel()

	seed := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.cozystack.io/v1alpha1",
		"kind":       "Tenant",
		"metadata":   map[string]any{"name": "dev", "namespace": "tenant-root"},
		"status": map[string]any{
			"conditions": []any{map[string]any{"type": "Ready", "status": "True"}},
		},
	}}

	c := client.New(newFake(seed))

	got, err := c.WaitForReady(context.Background(), client.TenantResource(), "tenant-root", "dev", time.Minute)
	if err != nil {
		t.Fatalf("WaitForReady() error = %v", err)
	}

	if !got.Status.Ready {
		t.Errorf("Status.Ready = false, want true")
	}
}

func TestWaitForReady_TimesOut(t *testing.T) {
	t.Parallel()

	seed := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.cozystack.io/v1alpha1",
		"kind":       "Tenant",
		"metadata":   map[string]any{"name": "dev", "namespace": "tenant-root"},
		"status": map[string]any{
			"conditions": []any{map[string]any{"type": "Ready", "status": "False"}},
		},
	}}

	c := client.New(newFake(seed))

	_, err := c.WaitForReady(context.Background(), client.TenantResource(), "tenant-root", "dev", 50*time.Millisecond)
	if err == nil {
		t.Fatal("WaitForReady() error = nil, want timeout error")
	}
}
