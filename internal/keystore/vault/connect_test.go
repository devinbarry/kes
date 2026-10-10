// Copyright 2026 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package vault

import (
	"context"
	"strings"
	"testing"
)

// Connect must reject these configs before it contacts Vault.
func TestConnect_RejectsUnsupportedConfig(t *testing.T) {
	valid := func() *Config {
		return &Config{
			Endpoint:   "https://vault.example.test:8200",
			APIVersion: APIv2,
			AppRole:    &AppRole{ID: "role-id", Secret: "secret-id"},
		}
	}
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string // a substring of the validation error
	}{
		{"http endpoint", func(c *Config) { c.Endpoint = "http://vault.example.test:8200" }, "https://"},
		{"unix endpoint", func(c *Config) { c.Endpoint = "unix:///run/vault.sock" }, "https://"},
		{"endpoint without host", func(c *Config) { c.Endpoint = "https://" }, "https://"},
		{"empty endpoint", func(c *Config) { c.Endpoint = "" }, "https://"},
		{"kv v1", func(c *Config) { c.APIVersion = APIv1 }, "API version"},
		{"missing kv version", func(c *Config) { c.APIVersion = "" }, "API version"},
		{"missing approle", func(c *Config) { c.AppRole = nil }, "AppRole"},
		{"missing secret id", func(c *Config) { c.AppRole.Secret = "" }, "AppRole"},
	}
	for _, test := range tests {
		c := valid()
		test.mutate(c)
		_, err := Connect(context.Background(), c)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: got error %v - want a validation error containing %q", test.name, err, test.want)
		}
	}
}
