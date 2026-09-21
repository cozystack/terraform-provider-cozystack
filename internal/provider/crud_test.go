package provider

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testBucketInfo = `{"spec":{"bucketName":"bucket-abc","secretS3":{"endpoint":"https://s3.example.test",` +
	`"region":"us-east-1","accessKeyID":"AKIDEXAMPLE","accessSecretKey":"shhh"}}}`

func testSecretsGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
}

// bucketWithUser is a bucket model expecting credentials for a single user.
func bucketWithUser() *bucketModel {
	return &bucketModel{
		Name:      types.StringValue("assets"),
		Namespace: types.StringValue("tenant-root"),
		Users: types.MapValueMust(types.ObjectType{AttrTypes: bucketUserObjectType()}, map[string]attr.Value{
			"reader": types.ObjectValueMust(bucketUserObjectType(), map[string]attr.Value{"readonly": types.BoolValue(true)}),
		}),
	}
}

func TestPollOutputs_WaitsForLateSecret(t *testing.T) {
	dyn := fakeOutputsDynamic()
	api := client.New(dyn)
	model := bucketWithUser()

	created := make(chan error, 1)

	go func() {
		time.Sleep(20 * time.Millisecond)

		secret := fakeSecret("tenant-root", "bucket-assets-reader", map[string]string{"BucketInfo": testBucketInfo})
		_, err := dyn.Resource(testSecretsGVR()).Namespace("tenant-root").
			Create(context.Background(), secret, metav1.CreateOptions{})
		created <- err
	}()

	diags := pollOutputs(context.Background(), model, api, time.Now().Add(10*time.Second), 5*time.Millisecond)

	if err := <-created; err != nil {
		t.Fatalf("creating the secret: %v", err)
	}

	if diags.HasError() {
		t.Fatalf("pollOutputs diagnostics: %v", diags)
	}

	if diags.WarningsCount() != 0 {
		t.Errorf("warnings = %d, want none once the secret appears", diags.WarningsCount())
	}

	if model.outputsPending() {
		t.Errorf("credentials = %v, want the late secret picked up", model.Credentials)
	}
}

func TestPollOutputs_TimesOutWithWarning(t *testing.T) {
	t.Parallel()

	api := fakeOutputsClient()
	model := bucketWithUser()

	diags := pollOutputs(context.Background(), model, api, time.Now().Add(15*time.Millisecond), 5*time.Millisecond)

	if diags.HasError() {
		t.Fatalf("pollOutputs diagnostics: %v", diags)
	}

	if diags.WarningsCount() != 1 {
		t.Errorf("warnings = %d, want one when outputs never appear", diags.WarningsCount())
	}

	if !model.Credentials.IsNull() {
		t.Errorf("credentials = %v, want null when the secret never appears", model.Credentials)
	}
}

func TestPollOutputs_StopsOnPersistentReadError(t *testing.T) {
	t.Parallel()

	dyn := fakeOutputsDynamic()
	dyn.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})

	model := bucketWithUser()

	start := time.Now()
	diags := pollOutputs(context.Background(), model, client.New(dyn), time.Now().Add(time.Minute), time.Millisecond)

	if !diags.HasError() {
		t.Fatal("pollOutputs diagnostics = none, want the read error reported")
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("elapsed = %s, want a return well before the deadline", elapsed)
	}
}

// The budget counts consecutive failures: a read that succeeds between two
// failures puts the whole budget back.
func TestPollOutputs_RetryBudgetCountsConsecutiveFailures(t *testing.T) {
	t.Parallel()

	dyn := fakeOutputsDynamic()

	var reads atomic.Int32

	// Fail, succeed with the Secret still absent, then fail for good: the budget
	// is spent only by the last three.
	dyn.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		if reads.Add(1) == 2 {
			return false, nil, nil
		}

		return true, nil, errors.New("boom")
	})

	model := bucketWithUser()

	diags := pollOutputs(context.Background(), model, client.New(dyn), time.Now().Add(10*time.Second), time.Millisecond)

	if !diags.HasError() {
		t.Fatalf("pollOutputs diagnostics = %v, want the error once failures ran consecutively", diags)
	}

	if got := reads.Load(); got != 5 {
		t.Errorf("reads = %d, want five: the successful read puts the budget back", got)
	}
}

// A read that fails as the deadline passes has not used up the retry budget, so
// it must not escape as an error either.
func TestPollOutputs_ReadErrorAtDeadlineOnlyWarns(t *testing.T) {
	t.Parallel()

	dyn := fakeOutputsDynamic()
	dyn.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})

	model := bucketWithUser()

	diags := pollOutputs(context.Background(), model, client.New(dyn), time.Now().Add(time.Millisecond), time.Millisecond)

	if diags.HasError() {
		t.Fatalf("pollOutputs diagnostics = %v, want the timeout warning alone", diags)
	}

	if diags.WarningsCount() != 2 {
		t.Fatalf("warnings = %d, want the timeout warning and the read failure behind it", diags.WarningsCount())
	}

	if detail := diags.Warnings()[1].Detail(); !strings.Contains(detail, "boom") {
		t.Errorf("second warning = %q, want the failing read reported", detail)
	}
}

// A converging cluster answers with the odd 5xx, and the object is already
// created by then, so a single failing read must not fail the apply.
func TestPollOutputs_SurvivesTransientReadError(t *testing.T) {
	t.Parallel()

	dyn := fakeOutputsDynamic(fakeSecret("tenant-root", "bucket-assets-reader", map[string]string{
		"BucketInfo": testBucketInfo,
	}))

	var reads atomic.Int32

	dyn.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		if reads.Add(1) <= outputsReadAttempts-1 {
			return true, nil, errors.New("boom")
		}

		return false, nil, nil
	})

	model := bucketWithUser()

	diags := pollOutputs(context.Background(), model, client.New(dyn), time.Now().Add(10*time.Second), time.Millisecond)

	if diags.HasError() || diags.WarningsCount() != 0 {
		t.Fatalf("pollOutputs diagnostics: %v", diags)
	}

	if model.outputsPending() {
		t.Error("outputsPending() = true, want the credentials read after the failures stopped")
	}
}

func TestPollOutputs_ReturnsEarlyOnCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	model := bucketWithUser()

	start := time.Now()

	diags := pollOutputs(ctx, model, fakeOutputsClient(), time.Now().Add(time.Minute), 30*time.Second)
	if diags.HasError() {
		t.Fatalf("pollOutputs diagnostics: %v", diags)
	}

	if elapsed := time.Since(start); elapsed >= 30*time.Second {
		t.Errorf("elapsed = %s, want a return without sleeping out the interval", elapsed)
	}
}

// The wait is driven by the resource models registered in provider.go, which
// embed the plain models rather than being them.
var (
	_ outputsReader = (*bucketResourceModel)(nil)
	_ outputsReader = (*kubernetesResourceModel)(nil)
	_ outputsReader = (*postgresqlResourceModel)(nil)
	_ outputsReader = (*vminstanceResourceModel)(nil)
)

// modelWithoutOutputs stands in for the Kinds whose whole state comes from the
// aggregated API.
type modelWithoutOutputs struct{}

func TestReadOutputsAfterPersist_ModelWithoutOutputs(t *testing.T) {
	t.Parallel()

	var diags diag.Diagnostics

	readOutputsAfterPersist(
		context.Background(), &modelWithoutOutputs{}, fakeOutputsClient(), true, time.Now().Add(time.Minute), &diags,
	)

	if diags.HasError() || diags.WarningsCount() != 0 {
		t.Errorf("diagnostics = %v, want none for a model with no outputs", diags)
	}
}

func TestReadOutputsAfterPersist_WithoutWaitReadsOnce(t *testing.T) {
	t.Parallel()

	var diags diag.Diagnostics

	api := fakeOutputsClient(fakeSecret("tenant-root", "bucket-assets-reader", map[string]string{
		"BucketInfo": testBucketInfo,
	}))
	model := bucketWithUser()

	start := time.Now()
	readOutputsAfterPersist(context.Background(), model, api, false, time.Now().Add(time.Minute), &diags)

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("elapsed = %s, want a single read when wait_for_ready is off", elapsed)
	}

	if diags.WarningsCount() != 0 {
		t.Errorf("warnings = %d, want none when wait_for_ready is off", diags.WarningsCount())
	}

	if model.outputsPending() {
		t.Error("outputsPending() = true, want the existing credentials read even without waiting")
	}
}

// Adding a user to an existing bucket is the update form of the same race: the
// new user's Secret is published after the object is written.
func TestPollOutputs_UpdateWaitsForAddedUser(t *testing.T) {
	dyn := fakeOutputsDynamic(fakeSecret("tenant-root", "bucket-assets-reader", map[string]string{
		"BucketInfo": testBucketInfo,
	}))

	model := bucketWithUser()
	model.Users = types.MapValueMust(types.ObjectType{AttrTypes: bucketUserObjectType()}, map[string]attr.Value{
		"reader": types.ObjectValueMust(bucketUserObjectType(), map[string]attr.Value{"readonly": types.BoolValue(true)}),
		"writer": types.ObjectValueMust(bucketUserObjectType(), map[string]attr.Value{"readonly": types.BoolValue(false)}),
	})

	created := make(chan error, 1)

	go func() {
		time.Sleep(20 * time.Millisecond)

		secret := fakeSecret("tenant-root", "bucket-assets-writer", map[string]string{"BucketInfo": testBucketInfo})
		_, err := dyn.Resource(testSecretsGVR()).Namespace("tenant-root").
			Create(context.Background(), secret, metav1.CreateOptions{})
		created <- err
	}()

	diags := pollOutputs(context.Background(), model, client.New(dyn), time.Now().Add(10*time.Second), 5*time.Millisecond)

	if err := <-created; err != nil {
		t.Fatalf("creating the secret: %v", err)
	}

	if diags.HasError() || diags.WarningsCount() != 0 {
		t.Fatalf("pollOutputs diagnostics: %v", diags)
	}

	if got := len(model.Credentials.Elements()); got != 2 {
		t.Errorf("credentials = %d entries, want both users", got)
	}
}

func TestOutputsPending(t *testing.T) {
	t.Parallel()

	credentialsType := types.ObjectType{AttrTypes: bucketCredentialsObjectType()}
	filled := bucketWithUser()
	filled.Credentials = types.MapValueMust(credentialsType, map[string]attr.Value{
		"reader": types.ObjectValueMust(bucketCredentialsObjectType(), map[string]attr.Value{
			"bucket_name": types.StringValue("bucket-abc"),
			"endpoint":    types.StringValue("https://s3.example.test"),
			"region":      types.StringValue("us-east-1"),
			"access_key":  types.StringValue("AKIDEXAMPLE"),
			"secret_key":  types.StringValue("shhh"),
		}),
	})

	endpoints := &postgresqlModel{Endpoints: types.ObjectValueMust(pgConnectionObjectType(), map[string]attr.Value{
		"host":      types.StringValue("postgres-db-rw.tenant-root.svc"),
		"read_host": types.StringNull(),
		"port":      types.Int64Value(5432),
	})}

	tests := []struct {
		name  string
		model outputsReader
		want  bool
	}{
		{name: "bucket user without credentials", model: bucketWithUser(), want: true},
		{name: "bucket user with credentials", model: filled, want: false},
		{name: "bucket without users", model: &bucketModel{}, want: false},
		{name: "kubernetes without kubeconfig", model: &kubernetesModel{}, want: true},
		{name: "kubernetes with kubeconfig", model: &kubernetesModel{Kubeconfig: types.StringValue("apiVersion: v1")}, want: false},
		{name: "postgresql without endpoints", model: &postgresqlModel{}, want: true},
		{name: "postgresql with endpoints", model: endpoints, want: false},
		{
			name:  "vminstance running without address",
			model: &vminstanceModel{RunStrategy: types.StringValue("Always")},
			want:  true,
		},
		{
			name:  "vminstance halted",
			model: &vminstanceModel{RunStrategy: types.StringValue("Halted")},
			want:  false,
		},
		{
			name:  "vminstance run once",
			model: &vminstanceModel{RunStrategy: types.StringValue("Once")},
			want:  true,
		},
		{
			name:  "vminstance rerun on failure",
			model: &vminstanceModel{RunStrategy: types.StringValue("RerunOnFailure")},
			want:  true,
		},
		{
			name: "vminstance running with address",
			model: &vminstanceModel{
				RunStrategy: types.StringValue("Always"),
				IPAddress:   types.StringValue("192.0.2.10"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.model.outputsPending(); got != tt.want {
				t.Errorf("outputsPending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWaitDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		wait      bool
		timeout   types.String
		want      time.Duration
		wantError bool
	}{
		{name: "not waiting ignores the timeout", wait: false, timeout: types.StringValue("nonsense")},
		{name: "empty timeout falls back to the default", wait: true, timeout: types.StringNull(), want: defaultWaitTimeout},
		{name: "explicit timeout", wait: true, timeout: types.StringValue("90s"), want: 90 * time.Second},
		{name: "malformed timeout", wait: true, timeout: types.StringValue("soon"), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, diags := waitDuration(tt.wait, tt.timeout)

			if diags.HasError() != tt.wantError {
				t.Fatalf("diagnostics = %v, wantError %v", diags, tt.wantError)
			}

			if !tt.wantError && got != tt.want {
				t.Errorf("waitDuration() = %s, want %s", got, tt.want)
			}
		})
	}
}

// A cluster slower than wait_timeout used to leave the object in Cozystack with
// no state at all, so the next apply hit AlreadyExists.
func TestPersistApplication_ReadinessTimeoutKeepsTheObject(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "apps.cozystack.io", Version: "v1alpha1", Resource: "buckets"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "BucketList"},
	)

	var diags diag.Diagnostics

	app := &client.Application{Name: "assets", Namespace: "tenant-root", Spec: map[string]any{}}

	result, ok := persistApplication(
		context.Background(), client.New(dyn), client.BucketResource(), true, app, true, 50*time.Millisecond, &diags,
	)

	if !ok {
		t.Fatal("persistApplication() ok = false, want the created object reported back")
	}

	if result.Name != "assets" || result.Namespace != "tenant-root" {
		t.Errorf("result = %s/%s, want tenant-root/assets", result.Namespace, result.Name)
	}

	if !diags.HasError() {
		t.Error("diagnostics = none, want the readiness timeout reported")
	}
}
