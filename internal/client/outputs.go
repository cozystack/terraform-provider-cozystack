package client

import (
	"context"
	"encoding/base64"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GVRs for the runtime artifacts that carry an application's outputs. These live
// outside the aggregated apps.cozystack.io API: the charts materialise
// connection details as core Secrets/Services and (for VMs) KubeVirt status.
func secretGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
}

func serviceGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
}

func vmiGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}
}

// ServiceEndpoint is the in-cluster address of a Service.
type ServiceEndpoint struct {
	Host string
	Port int64
}

// GetSecretData reads a Secret and returns its decoded data. found is false when
// the Secret does not exist yet; outputs are materialised asynchronously, so a
// missing Secret is a normal transient state, not an error.
func (c *Client) GetSecretData(ctx context.Context, namespace, name string) (map[string][]byte, bool, error) {
	obj, err := c.dyn.Resource(secretGVR()).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("reading secret %s/%s: %w", namespace, name, err)
	}

	raw, _, _ := unstructured.NestedMap(obj.Object, "data")
	out := make(map[string][]byte, len(raw))

	for key, value := range raw {
		encoded, ok := value.(string)
		if !ok {
			continue
		}

		decoded, decErr := base64.StdEncoding.DecodeString(encoded)
		if decErr != nil {
			return nil, false, fmt.Errorf("decoding secret key %q in %s/%s: %w", key, namespace, name, decErr)
		}

		out[key] = decoded
	}

	return out, true, nil
}

// GetServiceEndpoint returns the in-cluster DNS endpoint (`<name>.<namespace>.svc`)
// and first port of a Service. found is false when the Service does not exist yet.
func (c *Client) GetServiceEndpoint(ctx context.Context, namespace, name string) (ServiceEndpoint, bool, error) {
	obj, err := c.dyn.Resource(serviceGVR()).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return ServiceEndpoint{}, false, nil
	}

	if err != nil {
		return ServiceEndpoint{}, false, fmt.Errorf("reading service %s/%s: %w", namespace, name, err)
	}

	endpoint := ServiceEndpoint{Host: name + "." + namespace + ".svc"}

	ports, _, _ := unstructured.NestedSlice(obj.Object, "spec", "ports")
	if len(ports) > 0 {
		if port, ok := ports[0].(map[string]any); ok {
			endpoint.Port = toInt64(port["port"])
		}
	}

	return endpoint, true, nil
}

// GetVMIAddresses returns the IP addresses of a VirtualMachineInstance's first
// interface. found is false when the VMI does not exist yet; an existing VMI with
// no address yet returns an empty slice with found=true.
func (c *Client) GetVMIAddresses(ctx context.Context, namespace, name string) ([]string, bool, error) {
	obj, err := c.dyn.Resource(vmiGVR()).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("reading virtualmachineinstance %s/%s: %w", namespace, name, err)
	}

	interfaces, found, _ := unstructured.NestedSlice(obj.Object, "status", "interfaces")
	if !found || len(interfaces) == 0 {
		return nil, true, nil
	}

	first, ok := interfaces[0].(map[string]any)
	if !ok {
		return nil, true, nil
	}

	if list, ok := first["ipAddresses"].([]any); ok {
		addresses := make([]string, 0, len(list))

		for _, item := range list {
			if address, ok := item.(string); ok {
				addresses = append(addresses, address)
			}
		}

		return addresses, true, nil
	}

	if address, ok := first["ipAddress"].(string); ok {
		return []string{address}, true, nil
	}

	return nil, true, nil
}

// toInt64 coerces an unstructured numeric value (int64 or float64) to int64.
func toInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case float64:
		return int64(typed)
	case int:
		return int64(typed)
	default:
		return 0
	}
}
