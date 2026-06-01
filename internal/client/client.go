package client

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// fieldManager identifies this provider as the owner of server-side fields.
const fieldManager = "terraform-provider-cozystack"

// Client talks to the Cozystack aggregated API via the Kubernetes dynamic
// client. It exposes typed helpers for the resources the provider manages.
type Client struct {
	dyn dynamic.Interface
}

// New wraps an existing dynamic.Interface. It is the seam used by unit tests,
// which inject a fake dynamic client.
func New(dyn dynamic.Interface) *Client {
	return &Client{dyn: dyn}
}

// NewForConfig builds a Client from a *rest.Config.
func NewForConfig(restConfig *rest.Config) (*Client, error) {
	dyn, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return New(dyn), nil
}

// IsNotFound reports whether err is a Kubernetes "not found" API error,
// unwrapping wrapped errors as needed.
func IsNotFound(err error) bool {
	return apierrors.IsNotFound(err)
}
