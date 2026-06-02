package client

import (
	"context"
	"encoding/base64"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SecretObject is the provider-facing view of a Secret-shaped Cozystack kind
// (e.g. TenantSecret): the payload lives in top-level data, not under spec.
type SecretObject struct {
	Name            string
	Namespace       string
	UID             string
	ResourceVersion string
	Type            string
	Deleting        bool
	// Data holds the decoded values keyed by name.
	Data map[string][]byte
}

// CreateSecretObject creates a Secret-shaped object and returns the server view.
func (c *Client) CreateSecretObject(ctx context.Context, res Resource, obj *SecretObject) (SecretObject, error) {
	created, err := c.resource(res, obj.Namespace).
		Create(ctx, toSecretUnstructured(res, obj), metav1.CreateOptions{FieldManager: fieldManager})
	if err != nil {
		return SecretObject{}, fmt.Errorf("creating %s %s/%s: %w", res.Kind, obj.Namespace, obj.Name, err)
	}

	return secretFromUnstructured(created), nil
}

// GetSecretObject reads a Secret-shaped object. The error is left unwrapped so
// callers can test it with IsNotFound.
func (c *Client) GetSecretObject(ctx context.Context, res Resource, namespace, name string) (SecretObject, error) {
	got, err := c.resource(res, namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return SecretObject{}, err //nolint:wrapcheck // unwrapped so callers can use IsNotFound
	}

	return secretFromUnstructured(got), nil
}

// UpdateSecretObject replaces the type and data, retrying once on conflict.
func (c *Client) UpdateSecretObject(ctx context.Context, res Resource, obj *SecretObject) (SecretObject, error) {
	var lastErr error

	for range updateMaxRetries {
		current, err := c.resource(res, obj.Namespace).Get(ctx, obj.Name, metav1.GetOptions{})
		if err != nil {
			return SecretObject{}, fmt.Errorf("reading %s %s/%s before update: %w", res.Kind, obj.Namespace, obj.Name, err)
		}

		current.Object["type"] = obj.Type
		current.Object["data"] = encodeSecretData(obj.Data)
		delete(current.Object, "stringData")

		updated, err := c.resource(res, obj.Namespace).Update(ctx, current, metav1.UpdateOptions{FieldManager: fieldManager})
		if err == nil {
			return secretFromUnstructured(updated), nil
		}

		if !apierrors.IsConflict(err) {
			return SecretObject{}, fmt.Errorf("updating %s %s/%s: %w", res.Kind, obj.Namespace, obj.Name, err)
		}

		lastErr = err
	}

	return SecretObject{}, fmt.Errorf("updating %s %s/%s after retry: %w", res.Kind, obj.Namespace, obj.Name, lastErr)
}

func toSecretUnstructured(res Resource, obj *SecretObject) *unstructured.Unstructured {
	object := map[string]any{
		"apiVersion": res.apiVersion(),
		"kind":       res.Kind,
		"metadata":   map[string]any{"name": obj.Name, "namespace": obj.Namespace},
		"data":       encodeSecretData(obj.Data),
	}

	if obj.Type != "" {
		object["type"] = obj.Type
	}

	return &unstructured.Unstructured{Object: object}
}

func encodeSecretData(data map[string][]byte) map[string]any {
	out := make(map[string]any, len(data))
	for key, value := range data {
		out[key] = base64.StdEncoding.EncodeToString(value)
	}

	return out
}

func secretFromUnstructured(obj *unstructured.Unstructured) SecretObject {
	out := SecretObject{
		Name:            obj.GetName(),
		Namespace:       obj.GetNamespace(),
		UID:             string(obj.GetUID()),
		ResourceVersion: obj.GetResourceVersion(),
		Deleting:        obj.GetDeletionTimestamp() != nil,
		Data:            map[string][]byte{},
	}

	out.Type, _, _ = unstructured.NestedString(obj.Object, "type")

	raw, _, _ := unstructured.NestedMap(obj.Object, "data")
	for key, value := range raw {
		encoded, ok := value.(string)
		if !ok {
			continue
		}

		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}

		out.Data[key] = decoded
	}

	return out
}
