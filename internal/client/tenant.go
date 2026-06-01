package client

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	tenantAPIVersion  = "apps.cozystack.io/v1alpha1"
	tenantKind        = "Tenant"
	updateMaxRetries  = 2
	readyPollInterval = 5 * time.Second
)

// tenantGVR is the namespaced GroupVersionResource for Cozystack tenants.
//
//nolint:gochecknoglobals // immutable GroupVersionResource for the Tenant kind
var tenantGVR = schema.GroupVersionResource{
	Group:    "apps.cozystack.io",
	Version:  "v1alpha1",
	Resource: "tenants",
}

// Tenant is the provider-facing view of a Cozystack Tenant. Spec is the
// free-form application spec; only the keys the provider manages are populated.
type Tenant struct {
	Name            string
	Namespace       string
	ResourceVersion string
	Spec            map[string]any
	Status          TenantStatus
}

// TenantStatus carries the computed status fields surfaced to Terraform.
type TenantStatus struct {
	Namespace string
	Version   string
	Ready     bool
}

// CreateTenant creates a Tenant and returns the server view of it.
func (c *Client) CreateTenant(ctx context.Context, tenant *Tenant) (Tenant, error) {
	created, err := c.dyn.Resource(tenantGVR).Namespace(tenant.Namespace).
		Create(ctx, tenantToUnstructured(tenant), metav1.CreateOptions{FieldManager: fieldManager})
	if err != nil {
		return Tenant{}, fmt.Errorf("creating tenant %s/%s: %w", tenant.Namespace, tenant.Name, err)
	}

	return unstructuredToTenant(created), nil
}

// GetTenant reads a Tenant. The returned error is left unwrapped so callers can
// test it with IsNotFound.
func (c *Client) GetTenant(ctx context.Context, namespace, name string) (Tenant, error) {
	got, err := c.dyn.Resource(tenantGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return Tenant{}, err //nolint:wrapcheck // unwrapped so callers can use IsNotFound
	}

	return unstructuredToTenant(got), nil
}

// UpdateTenant replaces the Tenant spec. It reads the current object for its
// resourceVersion and retries once on an optimistic-concurrency conflict.
func (c *Client) UpdateTenant(ctx context.Context, tenant *Tenant) (Tenant, error) {
	var lastErr error

	for range updateMaxRetries {
		current, err := c.dyn.Resource(tenantGVR).Namespace(tenant.Namespace).
			Get(ctx, tenant.Name, metav1.GetOptions{})
		if err != nil {
			return Tenant{}, fmt.Errorf("reading tenant %s/%s before update: %w", tenant.Namespace, tenant.Name, err)
		}

		current.Object["spec"] = tenant.Spec

		updated, err := c.dyn.Resource(tenantGVR).Namespace(tenant.Namespace).
			Update(ctx, current, metav1.UpdateOptions{FieldManager: fieldManager})
		if err == nil {
			return unstructuredToTenant(updated), nil
		}

		if !apierrors.IsConflict(err) {
			return Tenant{}, fmt.Errorf("updating tenant %s/%s: %w", tenant.Namespace, tenant.Name, err)
		}

		lastErr = err
	}

	return Tenant{}, fmt.Errorf("updating tenant %s/%s after retry: %w", tenant.Namespace, tenant.Name, lastErr)
}

// WaitForTenantReady polls the tenant until its Ready condition is true or the
// timeout elapses, returning the most recent observation.
func (c *Client) WaitForTenantReady(
	ctx context.Context,
	namespace, name string,
	timeout time.Duration,
) (Tenant, error) {
	var last Tenant

	condition := func(ctx context.Context) (bool, error) {
		tenant, err := c.GetTenant(ctx, namespace, name)
		if err != nil {
			return false, err
		}

		last = tenant

		return tenant.Status.Ready, nil
	}

	err := wait.PollUntilContextTimeout(ctx, readyPollInterval, timeout, true, condition)
	if err != nil {
		return last, fmt.Errorf("waiting for tenant %s/%s to become ready: %w", namespace, name, err)
	}

	return last, nil
}

// DeleteTenant deletes a Tenant. A missing object is treated as success.
func (c *Client) DeleteTenant(ctx context.Context, namespace, name string) error {
	err := c.dyn.Resource(tenantGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting tenant %s/%s: %w", namespace, name, err)
	}

	return nil
}

// tenantToUnstructured renders a Tenant as an unstructured Cozystack object.
func tenantToUnstructured(tenant *Tenant) *unstructured.Unstructured {
	spec := tenant.Spec
	if spec == nil {
		spec = map[string]any{}
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": tenantAPIVersion,
		"kind":       tenantKind,
		"metadata":   map[string]any{"name": tenant.Name},
		"spec":       spec,
	}}

	if tenant.Namespace != "" {
		obj.SetNamespace(tenant.Namespace)
	}

	if tenant.ResourceVersion != "" {
		obj.SetResourceVersion(tenant.ResourceVersion)
	}

	return obj
}

// unstructuredToTenant projects a server object back into the provider view.
func unstructuredToTenant(obj *unstructured.Unstructured) Tenant {
	tenant := Tenant{
		Name:            obj.GetName(),
		Namespace:       obj.GetNamespace(),
		ResourceVersion: obj.GetResourceVersion(),
		Status:          extractStatus(obj),
	}

	if spec, found, _ := unstructured.NestedMap(obj.Object, "spec"); found {
		tenant.Spec = spec
	}

	return tenant
}

// extractStatus reads the computed status fields surfaced to Terraform.
func extractStatus(obj *unstructured.Unstructured) TenantStatus {
	namespace, _, _ := unstructured.NestedString(obj.Object, "status", "namespace")
	version, _, _ := unstructured.NestedString(obj.Object, "status", "version")

	return TenantStatus{
		Namespace: namespace,
		Version:   version,
		Ready:     readyFromConditions(obj),
	}
}

// readyFromConditions reports whether the object has a Ready=True condition.
func readyFromConditions(obj *unstructured.Unstructured) bool {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return false
	}

	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		if condition["type"] == "Ready" && condition["status"] == "True" {
			return true
		}
	}

	return false
}
