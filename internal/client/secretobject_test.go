package client_test

import (
	"context"
	"testing"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newSecretFake(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "core.cozystack.io", Version: "v1alpha1", Resource: "tenantsecrets"}: "TenantSecretList",
	}

	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
}

func TestSecretObject_RoundTrip(t *testing.T) {
	t.Parallel()

	c := client.New(newSecretFake())
	res := client.TenantSecretResource()

	in := client.SecretObject{
		Name:      "creds",
		Namespace: "tenant-root",
		Type:      "Opaque",
		Data:      map[string][]byte{"password": []byte("s3cr3t"), "user": []byte("admin")},
	}

	created, err := c.CreateSecretObject(context.Background(), res, &in)
	if err != nil {
		t.Fatalf("CreateSecretObject() error = %v", err)
	}

	if created.Type != "Opaque" {
		t.Errorf("type = %q, want Opaque", created.Type)
	}

	got, err := c.GetSecretObject(context.Background(), res, "tenant-root", "creds")
	if err != nil {
		t.Fatalf("GetSecretObject() error = %v", err)
	}

	if string(got.Data["password"]) != "s3cr3t" || string(got.Data["user"]) != "admin" {
		t.Errorf("data = %v, want decoded password/user (base64 round-trip)", got.Data)
	}
}
