package client_test

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

func TestWaitForReady_RetriesTransientReadErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "service unavailable", err: apierrors.NewServiceUnavailable("temporarily unavailable")},
		{name: "internal error", err: apierrors.NewInternalError(errors.New("temporary failure"))},
		{name: "server timeout", err: apierrors.NewServerTimeout(schema.GroupResource{Resource: "tenants"}, "get", 1)},
		{name: "timeout", err: apierrors.NewTimeoutError("request timed out", 1)},
		{name: "too many requests", err: apierrors.NewTooManyRequests("try again", 1)},
		{name: "connection closed", err: io.EOF},
		{name: "connection reset", err: &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}},
		{name: "connection refused", err: &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}},
		{name: "network timeout", err: &net.DNSError{IsTimeout: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seed := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "apps.cozystack.io/v1alpha1",
				"kind":       "Tenant",
				"metadata":   map[string]any{"name": "dev", "namespace": "tenant-root"},
				"status": map[string]any{
					"conditions": []any{map[string]any{"type": "Ready", "status": "True"}},
				},
			}}
			dyn := newFake(seed)
			reads := 0
			dyn.PrependReactor("get", "tenants", func(k8stesting.Action) (bool, runtime.Object, error) {
				reads++
				if reads == 1 {
					return true, nil, tt.err
				}

				return false, nil, nil
			})

			got, err := client.New(dyn).WaitForReady(
				context.Background(), client.TenantResource(), "tenant-root", "dev", 30*time.Second,
			)
			if err != nil {
				t.Fatalf("WaitForReady() error = %v, want recovery after a transient failure", err)
			}
			if !got.Status.Ready || reads != 2 {
				t.Errorf("ready = %t, reads = %d, want ready after two reads", got.Status.Ready, reads)
			}
		})
	}
}

func TestWaitForReady_StopsOnPermanentReadErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "forbidden", err: apierrors.NewForbidden(schema.GroupResource{Resource: "tenants"}, "dev", errors.New("denied"))},
		{name: "unauthorized", err: apierrors.NewUnauthorized("unauthorized")},
		{name: "not found", err: apierrors.NewNotFound(schema.GroupResource{Resource: "tenants"}, "dev")},
		{name: "bad request", err: apierrors.NewBadRequest("invalid request")},
		{name: "unknown error", err: errors.New("unexpected failure")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dyn := newFake()
			reads := 0
			dyn.PrependReactor("get", "tenants", func(k8stesting.Action) (bool, runtime.Object, error) {
				reads++

				return true, nil, tt.err
			})

			_, err := client.New(dyn).WaitForReady(
				context.Background(), client.TenantResource(), "tenant-root", "dev", 50*time.Millisecond,
			)
			if !errors.Is(err, tt.err) || errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("WaitForReady() error = %v, want the original error without a timeout", err)
			}
			if reads != 1 {
				t.Errorf("reads = %d, want no retries", reads)
			}
		})
	}
}

func TestWaitForReady_TransientErrorsRespectTimeout(t *testing.T) {
	t.Parallel()

	dyn := newFake()
	dyn.PrependReactor("get", "tenants", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewServiceUnavailable("temporarily unavailable")
	})

	start := time.Now()
	_, err := client.New(dyn).WaitForReady(
		context.Background(), client.TenantResource(), "tenant-root", "dev", 50*time.Millisecond,
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForReady() error = %v, want the wait deadline", err)
	}
	if !strings.Contains(err.Error(), "temporarily unavailable") {
		t.Errorf("WaitForReady() error = %v, want the last failed read in the detail", err)
	}
	if elapsed := time.Since(start); elapsed >= 5*time.Second {
		t.Errorf("elapsed = %s, want the timeout to interrupt the poll interval", elapsed)
	}
}

func TestWaitForReady_CancellationStopsRetries(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dyn := newFake()
	reads := 0
	dyn.PrependReactor("get", "tenants", func(k8stesting.Action) (bool, runtime.Object, error) {
		reads++
		cancel()

		return true, nil, apierrors.NewServiceUnavailable("temporarily unavailable")
	})

	start := time.Now()
	_, err := client.New(dyn).WaitForReady(ctx, client.TenantResource(), "tenant-root", "dev", time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("WaitForReady() error = %v, want cancellation", err)
	}
	if reads != 1 {
		t.Errorf("reads = %d, want no reads after cancellation", reads)
	}
	if elapsed := time.Since(start); elapsed >= 5*time.Second {
		t.Errorf("elapsed = %s, want cancellation to interrupt the poll interval", elapsed)
	}
}

func TestWaitForReady_KeepsLastObservationAfterReadFailure(t *testing.T) {
	t.Parallel()

	seed := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps.cozystack.io/v1alpha1",
		"kind":       "Tenant",
		"metadata":   map[string]any{"name": "dev", "namespace": "tenant-root", "resourceVersion": "42"},
	}}
	dyn := newFake(seed)
	reads := 0
	dyn.PrependReactor("get", "tenants", func(k8stesting.Action) (bool, runtime.Object, error) {
		reads++
		if reads == 1 {
			return false, nil, nil
		}

		return true, nil, apierrors.NewServiceUnavailable("temporarily unavailable")
	})

	got, err := client.New(dyn).WaitForReady(
		context.Background(), client.TenantResource(), "tenant-root", "dev", 6*time.Second,
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForReady() error = %v, want the wait deadline", err)
	}
	if got.Name != "dev" || got.ResourceVersion != "42" || reads < 2 {
		t.Errorf("last observation = %+v, reads = %d, want the successful read retained after a failed read", got, reads)
	}
}
