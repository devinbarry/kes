// Copyright 2023 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package kesconf

import (
	"testing"
	"time"
)

func TestReadServerConfigYAML_CustomAPI(t *testing.T) {
	const (
		Filename = "./testdata/custom-api.yml"

		StatusPath     = "/v1/status"
		StatusTimeout  = 17 * time.Second
		MetricsPath    = "/v1/metrics"
		MetricsTimeout = 22 * time.Second
	)

	config, err := ReadFile(Filename)
	if err != nil {
		t.Fatalf("Failed to read file '%s': %v", Filename, err)
	}

	api, ok := config.API.Paths[StatusPath]
	if !ok {
		t.Fatalf("Invalid API config: missing API '%s'", StatusPath)
	}
	if api.Timeout != StatusTimeout {
		t.Fatalf("Invalid API config: invalid timeout for '%s': got '%v' - want '%v'", StatusPath, api.Timeout, StatusTimeout)
	}

	api, ok = config.API.Paths[MetricsPath]
	if !ok {
		t.Fatalf("Invalid API config: missing API '%s'", MetricsPath)
	}
	if api.Timeout != MetricsTimeout {
		t.Fatalf("Invalid API config: invalid timeout for '%s': got '%v' - want '%v'", StatusPath, api.Timeout, MetricsTimeout)
	}
}

func TestReadServerConfigYAML_VaultWithAppRole(t *testing.T) {
	const (
		Filename = "./testdata/vault-approle.yml"

		Endpoint      = "https://127.0.0.1:8200"
		Engine        = "kv"
		APIVersion    = "v2"
		Namespace     = "ns1"
		Prefix        = "tenant-1"
		AppRoleEngine = "approle"
		AppRoleID     = "db02de05-fa39-4855-059b-67221c5c2f63"
		AppRoleSecret = "6a174c20-f6de-a53c-74d2-6018fcceff64"
	)

	config, err := ReadFile(Filename)
	if err != nil {
		t.Fatalf("Failed to read file '%s': %v", Filename, err)
	}

	vault, ok := config.KeyStore.(*VaultKeyStore)
	if !ok {
		var want *VaultKeyStore
		t.Fatalf("Invalid keystore: got type '%T' - want type '%T'", config.KeyStore, want)
	}
	if vault.Endpoint != Endpoint {
		t.Fatalf("Invalid endpoint: got '%s' - want '%s'", vault.Endpoint, Endpoint)
	}
	if vault.Engine != Engine {
		t.Fatalf("Invalid engine: got '%s' - want '%s'", vault.Engine, Engine)
	}
	if vault.APIVersion != APIVersion {
		t.Fatalf("Invalid API version: got '%s' - want '%s'", vault.APIVersion, APIVersion)
	}
	if vault.Namespace != Namespace {
		t.Fatalf("Invalid namespace: got '%s' - want '%s'", vault.Namespace, Namespace)
	}
	if vault.AppRole.Engine != AppRoleEngine {
		t.Fatalf("Invalid approle engine: got '%s' - want '%s'", vault.AppRole.Engine, AppRoleEngine)
	}
	if vault.AppRole.ID != AppRoleID {
		t.Fatalf("Invalid approle ID: got '%s' - want '%s'", vault.AppRole.ID, AppRoleID)
	}
	if vault.AppRole.Secret != AppRoleSecret {
		t.Fatalf("Invalid approle secret: got '%s' - want '%s'", vault.AppRole.Secret, AppRoleSecret)
	}
}
