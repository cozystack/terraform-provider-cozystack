package client_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newOutputsFake(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "secrets"}:                            "SecretList",
		{Group: "", Version: "v1", Resource: "services"}:                           "ServiceList",
		{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}: "VirtualMachineInstanceList",
	}

	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
}

func secretObject(namespace, name string, data map[string]string) *unstructured.Unstructured {
	encoded := map[string]any{}
	for key, value := range data {
		encoded[key] = base64.StdEncoding.EncodeToString([]byte(value))
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"namespace": namespace, "name": name},
		"data":       encoded,
	}}
}

func TestGetSecretData_DecodesAndReportsAbsence(t *testing.T) {
	t.Parallel()

	c := client.New(newOutputsFake(secretObject("tenant-root", "pg-app", map[string]string{
		"host":     "pg-rw.tenant-root.svc",
		"password": "s3cr3t",
	})))

	data, found, err := c.GetSecretData(context.Background(), "tenant-root", "pg-app")
	if err != nil || !found {
		t.Fatalf("GetSecretData() found=%v err=%v", found, err)
	}

	if string(data["password"]) != "s3cr3t" || string(data["host"]) != "pg-rw.tenant-root.svc" {
		t.Errorf("decoded data = %v, want host+password", data)
	}

	_, found, err = c.GetSecretData(context.Background(), "tenant-root", "absent")
	if err != nil || found {
		t.Errorf("absent secret: found=%v err=%v, want found=false err=nil", found, err)
	}
}

func TestGetServiceEndpoint(t *testing.T) {
	t.Parallel()

	svc := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]any{"namespace": "tenant-root", "name": "pg-rw"},
		"spec":       map[string]any{"ports": []any{map[string]any{"port": int64(5432)}}},
	}}

	c := client.New(newOutputsFake(svc))

	endpoint, found, err := c.GetServiceEndpoint(context.Background(), "tenant-root", "pg-rw")
	if err != nil || !found {
		t.Fatalf("GetServiceEndpoint() found=%v err=%v", found, err)
	}

	if endpoint.Host != "pg-rw.tenant-root.svc" || endpoint.Port != 5432 {
		t.Errorf("endpoint = %+v, want pg-rw.tenant-root.svc:5432", endpoint)
	}
}

func TestGetVMIAddresses(t *testing.T) {
	t.Parallel()

	vmi := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachineInstance",
		"metadata":   map[string]any{"namespace": "tenant-root", "name": "vm-instance-demo"},
		"status": map[string]any{
			"interfaces": []any{map[string]any{"ipAddresses": []any{"10.244.2.60", "fe80::1"}}},
		},
	}}

	c := client.New(newOutputsFake(vmi))

	addrs, found, err := c.GetVMIAddresses(context.Background(), "tenant-root", "vm-instance-demo")
	if err != nil || !found {
		t.Fatalf("GetVMIAddresses() found=%v err=%v", found, err)
	}

	if len(addrs) != 2 || addrs[0] != "10.244.2.60" {
		t.Errorf("addresses = %v, want [10.244.2.60 fe80::1]", addrs)
	}
}
