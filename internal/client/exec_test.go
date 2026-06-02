package client_test

import (
	"testing"

	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

func TestRestConfig_ExecAndClientCert(t *testing.T) {
	t.Parallel()

	cfg := client.Config{
		Host:              "https://api.example.test:6443",
		ClientCertificate: "-----BEGIN CERTIFICATE-----\ncert\n-----END CERTIFICATE-----",
		ClientKey:         "-----BEGIN PRIVATE KEY-----\nkey\n-----END PRIVATE KEY-----",
		Exec: &client.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1",
			Command:    "kubectl",
			Args:       []string{"oidc-login", "get-token"},
			Env:        map[string]string{"OIDC_ISSUER": "https://issuer.example.test"},
		},
	}

	rc, err := cfg.RestConfig()
	if err != nil {
		t.Fatalf("RestConfig() error = %v", err)
	}

	if rc.ExecProvider == nil {
		t.Fatal("ExecProvider is nil, want the exec credential plugin set")
	}

	if rc.ExecProvider.Command != "kubectl" || len(rc.ExecProvider.Args) != 2 {
		t.Errorf("ExecProvider = %+v, want kubectl with two args", rc.ExecProvider)
	}

	if len(rc.ExecProvider.Env) != 1 || rc.ExecProvider.Env[0].Name != "OIDC_ISSUER" {
		t.Errorf("ExecProvider.Env = %v, want one OIDC_ISSUER var", rc.ExecProvider.Env)
	}

	if string(rc.CertData) == "" || string(rc.KeyData) == "" {
		t.Errorf("client cert/key not set on TLSClientConfig")
	}
}
