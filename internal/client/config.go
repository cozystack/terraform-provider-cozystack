// Package client wraps the Kubernetes dynamic client used to talk to the
// Cozystack aggregated API (apps.cozystack.io).
package client

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// errNoHost is returned when no API server host can be resolved from any source.
var errNoHost = errors.New(
	"no Kubernetes API host resolved: set host, provide a kubeconfig, or enable in_cluster",
)

// Config holds the provider connection settings before they are turned into a
// *rest.Config. Empty fields are filled from the environment by ApplyEnvDefaults.
type Config struct {
	Host                 string
	Token                string
	ClusterCACertificate string
	Insecure             bool
	ConfigPath           string
	ConfigContext        string
	InCluster            bool
}

// ApplyEnvDefaults fills empty fields from the standard KUBE_* environment
// variables. Explicitly set fields always win over the environment.
func (c *Config) ApplyEnvDefaults() {
	c.ConfigPath = firstNonEmpty(c.ConfigPath, os.Getenv("KUBE_CONFIG_PATH"), os.Getenv("KUBECONFIG"))
	c.ConfigContext = firstNonEmpty(c.ConfigContext, os.Getenv("KUBE_CTX"))
	c.Host = firstNonEmpty(c.Host, os.Getenv("KUBE_HOST"))
	c.Token = firstNonEmpty(c.Token, os.Getenv("KUBE_TOKEN"))
	c.ClusterCACertificate = firstNonEmpty(c.ClusterCACertificate, os.Getenv("KUBE_CLUSTER_CA_CERT_DATA"))

	if !c.Insecure {
		v, err := strconv.ParseBool(os.Getenv("KUBE_INSECURE"))
		if err == nil {
			c.Insecure = v
		}
	}
}

// RestConfig builds a *rest.Config from c, applying the documented precedence:
// in-cluster, then kubeconfig (with optional context), then explicit overrides.
func (c *Config) RestConfig() (*rest.Config, error) {
	base, err := c.loadBaseConfig()
	if err != nil {
		return nil, err
	}

	c.applyOverrides(base)

	if base.Host == "" {
		return nil, errNoHost
	}

	return base, nil
}

// loadBaseConfig resolves the base connection without the explicit overrides.
func (c *Config) loadBaseConfig() (*rest.Config, error) {
	if c.InCluster {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("loading in-cluster config: %w", err)
		}

		return cfg, nil
	}

	// Skip kubeconfig entirely when the caller pins an explicit host and no
	// kubeconfig path: this keeps a host-only configuration deterministic.
	if c.ConfigPath == "" && c.Host != "" {
		return &rest.Config{}, nil
	}

	return loadKubeconfig(c.ConfigPath, c.ConfigContext)
}

// applyOverrides layers the explicit connection fields on top of base.
func (c *Config) applyOverrides(base *rest.Config) {
	if c.Host != "" {
		base.Host = c.Host
	}

	if c.Token != "" {
		base.BearerToken = c.Token
	}

	if c.ClusterCACertificate != "" {
		base.CAData = []byte(c.ClusterCACertificate)
		base.CAFile = ""
	}

	if c.Insecure {
		base.Insecure = true
		base.CAData = nil
		base.CAFile = ""
	}
}

// loadKubeconfig loads a kubeconfig from an explicit path (or the default
// loading rules when path is empty), honouring an optional context override.
func loadKubeconfig(path, context string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if path != "" {
		rules.ExplicitPath = path
	}

	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}
