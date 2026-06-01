package client_test

import (
	"path/filepath"
	"testing"

	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

func TestResolveEnv_FillsEmptyFieldsFromEnv(t *testing.T) {
	t.Setenv("KUBE_CONFIG_PATH", "/env/kubeconfig")
	t.Setenv("KUBE_CTX", "env-context")
	t.Setenv("KUBE_HOST", "https://env.example.test:6443")
	t.Setenv("KUBE_TOKEN", "env-token")
	t.Setenv("KUBE_CLUSTER_CA_CERT_DATA", "env-ca")
	t.Setenv("KUBE_INSECURE", "true")

	got := client.Config{}
	got.ApplyEnvDefaults()

	if got.ConfigPath != "/env/kubeconfig" {
		t.Errorf("ConfigPath = %q, want /env/kubeconfig", got.ConfigPath)
	}
	if got.ConfigContext != "env-context" {
		t.Errorf("ConfigContext = %q, want env-context", got.ConfigContext)
	}
	if got.Host != "https://env.example.test:6443" {
		t.Errorf("Host = %q, want env host", got.Host)
	}
	if got.Token != "env-token" {
		t.Errorf("Token = %q, want env-token", got.Token)
	}
	if got.ClusterCACertificate != "env-ca" {
		t.Errorf("ClusterCACertificate = %q, want env-ca", got.ClusterCACertificate)
	}
	if !got.Insecure {
		t.Errorf("Insecure = false, want true")
	}
}

func TestResolveEnv_ExplicitValuesWinOverEnv(t *testing.T) {
	t.Setenv("KUBE_HOST", "https://env.example.test:6443")
	t.Setenv("KUBE_TOKEN", "env-token")

	got := client.Config{
		Host:  "https://explicit.example.test:6443",
		Token: "explicit-token",
	}
	got.ApplyEnvDefaults()

	if got.Host != "https://explicit.example.test:6443" {
		t.Errorf("Host = %q, want explicit host", got.Host)
	}
	if got.Token != "explicit-token" {
		t.Errorf("Token = %q, want explicit-token", got.Token)
	}
}

func TestResolveEnv_KubeconfigEnvFallback(t *testing.T) {
	t.Setenv("KUBE_CONFIG_PATH", "")
	t.Setenv("KUBECONFIG", "/from/kubeconfig/env")

	got := client.Config{}
	got.ApplyEnvDefaults()

	if got.ConfigPath != "/from/kubeconfig/env" {
		t.Errorf("ConfigPath = %q, want /from/kubeconfig/env", got.ConfigPath)
	}
}

func TestRestConfig_ExplicitHostNoKubeconfig(t *testing.T) {
	cfg := client.Config{
		Host:  "https://explicit.example.test:6443",
		Token: "explicit-token",
	}

	rc, err := cfg.RestConfig()
	if err != nil {
		t.Fatalf("RestConfig() error = %v", err)
	}
	if rc.Host != "https://explicit.example.test:6443" {
		t.Errorf("Host = %q, want explicit host", rc.Host)
	}
	if rc.BearerToken != "explicit-token" {
		t.Errorf("BearerToken = %q, want explicit-token", rc.BearerToken)
	}
}

func TestRestConfig_FromKubeconfigCurrentContext(t *testing.T) {
	cfg := client.Config{ConfigPath: filepath.Join("testdata", "kubeconfig")}

	rc, err := cfg.RestConfig()
	if err != nil {
		t.Fatalf("RestConfig() error = %v", err)
	}
	if rc.Host != "https://api.example.test:6443" {
		t.Errorf("Host = %q, want current-context host", rc.Host)
	}
}

func TestRestConfig_ContextOverride(t *testing.T) {
	cfg := client.Config{
		ConfigPath:    filepath.Join("testdata", "kubeconfig"),
		ConfigContext: "other-context",
	}

	rc, err := cfg.RestConfig()
	if err != nil {
		t.Fatalf("RestConfig() error = %v", err)
	}
	if rc.Host != "https://other.example.test:6443" {
		t.Errorf("Host = %q, want other-context host", rc.Host)
	}
}

func TestRestConfig_ExplicitHostOverridesKubeconfig(t *testing.T) {
	cfg := client.Config{
		ConfigPath: filepath.Join("testdata", "kubeconfig"),
		Host:       "https://override.example.test:6443",
	}

	rc, err := cfg.RestConfig()
	if err != nil {
		t.Fatalf("RestConfig() error = %v", err)
	}
	if rc.Host != "https://override.example.test:6443" {
		t.Errorf("Host = %q, want override host", rc.Host)
	}
}

func TestRestConfig_InsecureClearsCA(t *testing.T) {
	cfg := client.Config{
		Host:                 "https://explicit.example.test:6443",
		Token:                "explicit-token",
		ClusterCACertificate: "ignored-ca",
		Insecure:             true,
	}

	rc, err := cfg.RestConfig()
	if err != nil {
		t.Fatalf("RestConfig() error = %v", err)
	}
	if !rc.Insecure {
		t.Errorf("Insecure = false, want true")
	}
	if len(rc.CAData) != 0 {
		t.Errorf("CAData = %q, want empty when insecure", rc.CAData)
	}
}

func TestRestConfig_NoHostNoKubeconfigErrors(t *testing.T) {
	cfg := client.Config{ConfigPath: filepath.Join("testdata", "does-not-exist")}

	_, err := cfg.RestConfig()
	if err == nil {
		t.Fatal("RestConfig() error = nil, want error when nothing resolvable")
	}
}
