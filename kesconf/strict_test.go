// Copyright 2026 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package kesconf

import (
	"os"
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

// The annotated sample config leaves credentials empty, so it is only
// checked for options this fork no longer knows.
func TestSampleServerConfigHasOnlyKnownFields(t *testing.T) {
	f, err := os.Open("../server-config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var y ymlFile
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&y); err != nil {
		t.Fatalf("Sample config contains an unknown option: %v", err)
	}
}

func TestReadServerConfigYAML_Production(t *testing.T) {
	const Identity = "3ecfcdf38fcbe141ae26a1030f81e96b753365a46760ae6b578698a97c59fd22"
	t.Setenv("MINIO_IDENTITY", Identity)

	config, err := ReadFile("./testdata/production.yml")
	if err != nil {
		t.Fatalf("Failed to read production config: %v", err)
	}
	vault, ok := config.KeyStore.(*VaultKeyStore)
	if !ok {
		t.Fatalf("Invalid keystore: got %T - want %T", config.KeyStore, &VaultKeyStore{})
	}
	if vault.Transit == nil || vault.Transit.KeyName != "minio-s3" {
		t.Fatalf("Invalid transit config: %+v", vault.Transit)
	}
	policy, ok := config.Policies["minio"]
	if !ok || len(policy.Identities) != 1 || policy.Identities[0] != Identity {
		t.Fatalf("Invalid minio policy: %+v", policy)
	}
}

func TestReadServerConfigYAML_RejectsUnknownFields(t *testing.T) {
	const (
		TLS      = "address: 0.0.0.0:7373\nadmin:\n  identity: disabled\ntls:\n  key: ./k\n  cert: ./c\n"
		Vault    = "  vault:\n    endpoint: https://127.0.0.1:8200\n    version: v2\n    approle:\n      id: a\n      secret: b\n"
		KeyStore = "keystore:\n" + Vault
	)
	tests := map[string]string{
		"removed filesystem keystore": TLS + "keystore:\n  fs:\n    path: /tmp/kes\n" + Vault,
		"removed aws keystore":        TLS + "keystore:\n  aws:\n    secretsmanager:\n      region: us-east-1\n" + Vault,
		"removed skip_auth":           TLS + "api:\n  /v1/metrics:\n    skip_auth: true\n" + KeyStore,
		"removed tls proxy":           TLS + "  proxy:\n    identities: []\n" + KeyStore,
		"misspelled vault option":     TLS + strings.Replace(KeyStore, "endpoint", "endpont", 1),
		"removed kubernetes auth":     TLS + KeyStore + "    kubernetes:\n      role: kes\n",
		"kv v1 engine":                TLS + strings.Replace(KeyStore, "version: v2", "version: v1", 1),
		"missing kv version":          TLS + strings.Replace(KeyStore, "    version: v2\n", "", 1),
	}
	if _, err := ReadFrom(strings.NewReader(TLS + KeyStore)); err != nil {
		t.Fatalf("Valid base config was rejected: %v", err)
	}
	for name, config := range tests {
		if _, err := ReadFrom(strings.NewReader(config)); err == nil {
			t.Errorf("%s: config was accepted", name)
		}
	}
}
