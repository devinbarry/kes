// Copyright 2026 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package vault

import (
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNewClientConfig_EnforcesTLS(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:3128")

	config, err := newClientConfig(&Config{Endpoint: "https://vault.example.test:8200"})
	if err != nil {
		t.Fatalf("Failed to create client config: %v", err)
	}
	tr := config.HttpClient.Transport.(*http.Transport)
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS13 {
		t.Fatalf("Invalid TLS min version: got %x - want %x", tr.TLSClientConfig.MinVersion, tls.VersionTLS13)
	}
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("Certificate verification is disabled")
	}
	if tr.Proxy != nil {
		t.Fatal("Vault traffic may be routed through an environment proxy")
	}
}

func TestNewClientConfig_RejectsVaultEnv(t *testing.T) {
	t.Setenv("VAULT_SKIP_VERIFY", "true")

	if _, err := newClientConfig(&Config{Endpoint: "https://vault.example.test:8200"}); err == nil {
		t.Fatal("Client config was created while VAULT_SKIP_VERIFY is set")
	}
}

func TestNewClientConfig_RejectsInvalidCA(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newClientConfig(&Config{Endpoint: "https://vault.example.test:8200", CAPath: caFile}); err == nil {
		t.Fatal("Client config was created with an invalid CA file")
	}
}

func TestRejectVaultEnv(t *testing.T) {
	if err := rejectVaultEnv([]string{"HOME=/root", "KES_SERVER=https://127.0.0.1:7373"}); err != nil {
		t.Fatalf("Unrelated variables were rejected: %v", err)
	}
	for _, kv := range []string{"VAULT_SKIP_VERIFY=true", "VAULT_CACERT=/tmp/ca.pem", "VAULT_TOKEN=", "VAULT_HEADERS={}"} {
		if err := rejectVaultEnv([]string{"HOME=/root", kv}); err == nil {
			t.Errorf("%s was accepted", kv)
		}
	}
}
