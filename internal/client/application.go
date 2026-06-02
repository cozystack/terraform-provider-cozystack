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
	"k8s.io/client-go/dynamic"
)

const (
	appGroup          = "apps.cozystack.io"
	appVersion        = "v1alpha1"
	updateMaxRetries  = 2
	readyPollInterval = 5 * time.Second
)

// Resource identifies one Cozystack kind. By default it is a namespaced kind in
// the apps.cozystack.io aggregated API; Group/Version/ClusterScoped override that
// for other groups (e.g. the cluster-scoped cozystack.io platform resources).
type Resource struct {
	// Resource is the lowercase plural name (e.g. "tenants", "redises").
	Resource string
	// Kind is the CamelCase kind (e.g. "Tenant", "Redis").
	Kind string
	// Group overrides the API group. Empty defaults to apps.cozystack.io.
	Group string
	// Version overrides the API version. Empty defaults to v1alpha1.
	Version string
	// ClusterScoped marks a non-namespaced kind.
	ClusterScoped bool
	// NoSpec marks a kind that has no spec (a marker resource); the spec field
	// is omitted on create/update.
	NoSpec bool
}

func (r Resource) group() string {
	if r.Group != "" {
		return r.Group
	}

	return appGroup
}

func (r Resource) version() string {
	if r.Version != "" {
		return r.Version
	}

	return appVersion
}

func (r Resource) apiVersion() string {
	return r.group() + "/" + r.version()
}

func (r Resource) gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: r.group(), Version: r.version(), Resource: r.Resource}
}

// resource returns the dynamic client scoped for this kind: namespaced for
// namespaced kinds, cluster-wide for cluster-scoped ones.
func (c *Client) resource(res Resource, namespace string) dynamic.ResourceInterface {
	if res.ClusterScoped {
		return c.dyn.Resource(res.gvr())
	}

	return c.dyn.Resource(res.gvr()).Namespace(namespace)
}

// Application is the provider-facing view of a Cozystack application. Spec is
// the free-form application spec; only the keys the provider manages are set.
type Application struct {
	Name            string
	Namespace       string
	UID             string
	ResourceVersion string
	// Deleting is true when the object has a deletion timestamp (terminating).
	Deleting bool
	Spec     map[string]any
	Status   ApplicationStatus
}

// ApplicationStatus carries the computed status surfaced to Terraform. Raw holds
// the full status object for kind-specific fields (e.g. a tenant's namespace).
type ApplicationStatus struct {
	Ready   bool
	Version string
	Raw     map[string]any
}

// Create creates an application and returns the server view of it.
func (c *Client) Create(ctx context.Context, res Resource, app *Application) (Application, error) {
	created, err := c.resource(res, app.Namespace).
		Create(ctx, toUnstructured(res, app), metav1.CreateOptions{FieldManager: fieldManager})
	if err != nil {
		return Application{}, fmt.Errorf("creating %s %s/%s: %w", res.Kind, app.Namespace, app.Name, err)
	}

	return fromUnstructured(created), nil
}

// Get reads an application. The error is left unwrapped so callers can test it
// with IsNotFound.
func (c *Client) Get(ctx context.Context, res Resource, namespace, name string) (Application, error) {
	got, err := c.resource(res, namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return Application{}, err //nolint:wrapcheck // unwrapped so callers can use IsNotFound
	}

	return fromUnstructured(got), nil
}

// Update replaces the application spec. It reads the current object for its
// resourceVersion and retries once on an optimistic-concurrency conflict.
func (c *Client) Update(ctx context.Context, res Resource, app *Application) (Application, error) {
	var lastErr error

	for range updateMaxRetries {
		current, err := c.resource(res, app.Namespace).
			Get(ctx, app.Name, metav1.GetOptions{})
		if err != nil {
			return Application{}, fmt.Errorf("reading %s %s/%s before update: %w", res.Kind, app.Namespace, app.Name, err)
		}

		if !res.NoSpec {
			current.Object["spec"] = app.Spec
		}

		updated, err := c.resource(res, app.Namespace).
			Update(ctx, current, metav1.UpdateOptions{FieldManager: fieldManager})
		if err == nil {
			return fromUnstructured(updated), nil
		}

		if !apierrors.IsConflict(err) {
			return Application{}, fmt.Errorf("updating %s %s/%s: %w", res.Kind, app.Namespace, app.Name, err)
		}

		lastErr = err
	}

	return Application{}, fmt.Errorf("updating %s %s/%s after retry: %w", res.Kind, app.Namespace, app.Name, lastErr)
}

// Delete deletes an application. A missing object is treated as success.
func (c *Client) Delete(ctx context.Context, res Resource, namespace, name string) error {
	err := c.resource(res, namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting %s %s/%s: %w", res.Kind, namespace, name, err)
	}

	return nil
}

// WaitForReady polls the application until its Ready condition is true or the
// timeout elapses, returning the most recent observation.
func (c *Client) WaitForReady(
	ctx context.Context,
	res Resource,
	namespace, name string,
	timeout time.Duration,
) (Application, error) {
	var last Application

	condition := func(ctx context.Context) (bool, error) {
		app, err := c.Get(ctx, res, namespace, name)
		if err != nil {
			return false, err
		}

		last = app

		return app.Status.Ready, nil
	}

	err := wait.PollUntilContextTimeout(ctx, readyPollInterval, timeout, true, condition)
	if err != nil {
		return last, fmt.Errorf("waiting for %s %s/%s to become ready: %w", res.Kind, namespace, name, err)
	}

	return last, nil
}

// toUnstructured renders an Application as an unstructured Cozystack object.
func toUnstructured(res Resource, app *Application) *unstructured.Unstructured {
	object := map[string]any{
		"apiVersion": res.apiVersion(),
		"kind":       res.Kind,
		"metadata":   map[string]any{"name": app.Name},
	}

	if !res.NoSpec {
		spec := app.Spec
		if spec == nil {
			spec = map[string]any{}
		}

		object["spec"] = spec
	}

	obj := &unstructured.Unstructured{Object: object}

	if app.Namespace != "" {
		obj.SetNamespace(app.Namespace)
	}

	if app.ResourceVersion != "" {
		obj.SetResourceVersion(app.ResourceVersion)
	}

	return obj
}

// fromUnstructured projects a server object back into the provider view.
func fromUnstructured(obj *unstructured.Unstructured) Application {
	app := Application{
		Name:            obj.GetName(),
		Namespace:       obj.GetNamespace(),
		UID:             string(obj.GetUID()),
		ResourceVersion: obj.GetResourceVersion(),
		Deleting:        obj.GetDeletionTimestamp() != nil,
		Status:          extractStatus(obj),
	}

	if spec, found, _ := unstructured.NestedMap(obj.Object, "spec"); found {
		app.Spec = spec
	}

	return app
}

// extractStatus reads the computed status fields surfaced to Terraform.
func extractStatus(obj *unstructured.Unstructured) ApplicationStatus {
	version, _, _ := unstructured.NestedString(obj.Object, "status", "version")
	raw, _, _ := unstructured.NestedMap(obj.Object, "status")

	return ApplicationStatus{
		Ready:   readyFromConditions(obj),
		Version: version,
		Raw:     raw,
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
