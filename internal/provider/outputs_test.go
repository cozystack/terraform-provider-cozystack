package provider

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// fakeOutputsClient builds a client backed by a fake dynamic client that serves
// Secrets, Services, and VirtualMachineInstances for outputs tests.
func fakeOutputsClient(objects ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "secrets"}:                            "SecretList",
		{Group: "", Version: "v1", Resource: "services"}:                           "ServiceList",
		{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}: "VirtualMachineInstanceList",
	}

	return client.New(dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...))
}

func fakeSecret(namespace, name string, data map[string]string) *unstructured.Unstructured {
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

func fakeService(namespace, name string, port int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]any{"namespace": namespace, "name": name},
		"spec":       map[string]any{"ports": []any{map[string]any{"port": port}}},
	}}
}

func fakeVMI(namespace, name string, addresses ...string) *unstructured.Unstructured {
	ips := make([]any, 0, len(addresses))
	for _, address := range addresses {
		ips = append(ips, address)
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachineInstance",
		"metadata":   map[string]any{"namespace": namespace, "name": name},
		"status":     map[string]any{"interfaces": []any{map[string]any{"ipAddresses": ips}}},
	}}
}

func TestKubernetesReadOutputs_Kubeconfig(t *testing.T) {
	t.Parallel()

	api := fakeOutputsClient(fakeSecret("tenant-root", "kubernetes-cluster-admin-kubeconfig", map[string]string{
		"super-admin.conf": "apiVersion: v1\nkind: Config\n",
	}))

	model := kubernetesModel{
		Name:      types.StringValue("cluster"),
		Namespace: types.StringValue("tenant-root"),
	}

	if diags := model.readOutputs(context.Background(), api); diags.HasError() {
		t.Fatalf("readOutputs diagnostics: %v", diags)
	}

	if model.Kubeconfig.ValueString() != "apiVersion: v1\nkind: Config\n" {
		t.Errorf("kubeconfig = %q, want the secret contents", model.Kubeconfig.ValueString())
	}
}

func TestKubernetesReadOutputs_AbsentSecretLeavesNull(t *testing.T) {
	t.Parallel()

	api := fakeOutputsClient()

	model := kubernetesModel{
		Name:      types.StringValue("cluster"),
		Namespace: types.StringValue("tenant-root"),
	}

	if diags := model.readOutputs(context.Background(), api); diags.HasError() {
		t.Fatalf("readOutputs diagnostics: %v", diags)
	}

	if !model.Kubeconfig.IsNull() {
		t.Errorf("kubeconfig = %q, want null when the secret is absent", model.Kubeconfig.ValueString())
	}
}

func TestVMInstanceReadOutputs_IPAddresses(t *testing.T) {
	t.Parallel()

	api := fakeOutputsClient(fakeVMI("tenant-root", "vm-instance-demo", "10.244.2.60", "fe80::1"))

	model := vminstanceModel{
		Name:      types.StringValue("demo"),
		Namespace: types.StringValue("tenant-root"),
	}

	if diags := model.readOutputs(context.Background(), api); diags.HasError() {
		t.Fatalf("readOutputs diagnostics: %v", diags)
	}

	if model.IPAddress.ValueString() != "10.244.2.60" {
		t.Errorf("ip_address = %q, want 10.244.2.60", model.IPAddress.ValueString())
	}

	if len(model.IPAddresses.Elements()) != 2 {
		t.Errorf("ip_addresses = %v, want two elements", model.IPAddresses.Elements())
	}
}

func TestPostgresqlReadOutputs_Connection(t *testing.T) {
	t.Parallel()

	api := fakeOutputsClient(
		fakeService("tenant-root", "postgres-db-rw", 5432),
		fakeService("tenant-root", "postgres-db-ro", 5432),
	)

	model := postgresqlModel{
		Name:      types.StringValue("db"),
		Namespace: types.StringValue("tenant-root"),
	}

	if diags := model.readOutputs(context.Background(), api); diags.HasError() {
		t.Fatalf("readOutputs diagnostics: %v", diags)
	}

	conn := model.Endpoints.Attributes()
	if conn["host"].(types.String).ValueString() != "postgres-db-rw.tenant-root.svc" {
		t.Errorf("endpoints.host = %v, want postgres-db-rw.tenant-root.svc", conn["host"])
	}

	if conn["read_host"].(types.String).ValueString() != "postgres-db-ro.tenant-root.svc" {
		t.Errorf("endpoints.read_host = %v, want postgres-db-ro.tenant-root.svc", conn["read_host"])
	}

	if conn["port"].(types.Int64).ValueInt64() != 5432 {
		t.Errorf("connection.port = %v, want 5432", conn["port"])
	}
}

func TestBucketReadOutputs_Credentials(t *testing.T) {
	t.Parallel()

	bucketInfoJSON := `{"spec":{"bucketName":"bucket-abc","secretS3":{"endpoint":"https://s3.example.test",` +
		`"region":"us-east-1","accessKeyID":"AKIDEXAMPLE","accessSecretKey":"shhh"}}}`

	api := fakeOutputsClient(fakeSecret("tenant-root", "bucket-assets-reader", map[string]string{
		"BucketInfo": bucketInfoJSON,
	}))

	model := bucketModel{
		Name:      types.StringValue("assets"),
		Namespace: types.StringValue("tenant-root"),
		Users: types.MapValueMust(types.ObjectType{AttrTypes: bucketUserObjectType()}, map[string]attr.Value{
			"reader": types.ObjectValueMust(bucketUserObjectType(), map[string]attr.Value{"readonly": types.BoolValue(true)}),
		}),
	}

	if diags := model.readOutputs(context.Background(), api); diags.HasError() {
		t.Fatalf("readOutputs diagnostics: %v", diags)
	}

	creds := model.Credentials.Elements()

	reader, ok := creds["reader"].(types.Object)
	if !ok {
		t.Fatalf("credentials missing reader: %v", creds)
	}

	attrs := reader.Attributes()
	if attrs["endpoint"].(types.String).ValueString() != "https://s3.example.test" {
		t.Errorf("endpoint = %v, want https://s3.example.test", attrs["endpoint"])
	}

	if attrs["secret_key"].(types.String).ValueString() != "shhh" {
		t.Errorf("secret_key = %v, want shhh", attrs["secret_key"])
	}
}
