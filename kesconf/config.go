// Copyright 2023 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package kesconf

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minio/kms-go/kes"
	"gopkg.in/yaml.v3"
)

type ymlFile struct {
	Version string `yaml:"version"`

	Addr env[string] `yaml:"address"`

	Admin struct {
		Identity env[kes.Identity] `yaml:"identity"`
	} `yaml:"admin"`

	TLS struct {
		PrivateKey  env[string] `yaml:"key"`
		Certificate env[string] `yaml:"cert"`
		CAPath      env[string] `yaml:"ca"`
		Password    env[string] `yaml:"password"`
		ClientAuth  env[string] `yaml:"auth"`
	} `yaml:"tls"`

	Policies map[string]struct {
		Allow      []string            `yaml:"allow"`
		Deny       []string            `yaml:"deny"`
		Identities []env[kes.Identity] `yaml:"identities"`
	} `yaml:"policy"`

	Cache struct {
		Expiry struct {
			Any     env[time.Duration] `yaml:"any"`
			Unused  env[time.Duration] `yaml:"unused"`
			Offline env[time.Duration] `yaml:"offline"`
		} `yaml:"expiry"`
	} `yaml:"cache"`

	API struct {
		Paths map[string]struct {
			Timeout env[time.Duration] `yaml:"timeout"`
		} `yaml:",inline"`
	} `yaml:"api"`

	Log struct {
		Error env[string] `yaml:"error"`
		Audit env[string] `yaml:"audit"`
	} `yaml:"log"`

	Keys []struct {
		Name env[string] `yaml:"name"`
	} `yaml:"keys"`

	KeyStore struct {
		Vault *struct {
			Endpoint   env[string] `yaml:"endpoint"`
			Engine     env[string] `yaml:"engine"`
			APIVersion env[string] `yaml:"version"`
			Namespace  env[string] `yaml:"namespace"`
			Prefix     env[string] `yaml:"prefix"`

			Transit *struct {
				Engine  env[string] `yaml:"engine"`
				KeyName env[string] `yaml:"key"`
			}

			AppRole *struct {
				Engine    env[string] `yaml:"engine"`
				Namespace env[string] `yaml:"namespace"`
				ID        env[string] `yaml:"id"`
				Secret    env[string] `yaml:"secret"`
			} `yaml:"approle"`

			Kubernetes *struct {
				Engine    env[string] `yaml:"engine"`
				Namespace env[string] `yaml:"namespace"`
				Role      env[string] `yaml:"role"`
				JWT       env[string] `yaml:"jwt"` // Can be either a JWT or a path to a file containing a JWT
			} `yaml:"kubernetes"`

			TLS struct {
				PrivateKey  env[string] `yaml:"key"`
				Certificate env[string] `yaml:"cert"`
				CAPath      env[string] `yaml:"ca"`
			} `yaml:"tls"`

			Status struct {
				Ping env[time.Duration] `yaml:"ping"`
			} `yaml:"status"`
		} `yaml:"vault"`
	} `yaml:"keystore"`
}

func findVersion(root *yaml.Node) (string, error) {
	if root == nil {
		return "", errors.New("kesconf: invalid config")
	}
	if root.Kind != yaml.DocumentNode {
		return "", errors.New("kesconf: invalid config format")
	}
	if len(root.Content) != 1 {
		return "", errors.New("kesconf: invalid config format")
	}

	doc := root.Content[0]
	for i, n := range doc.Content {
		if n.Value == "version" {
			if n.Kind != yaml.ScalarNode {
				return "", fmt.Errorf("kesconf: invalid config version at line '%d'", n.Line)
			}
			if i == len(doc.Content)-1 {
				return "", fmt.Errorf("kesconf: invalid config version at line '%d'", n.Line)
			}
			v := doc.Content[i+1]
			if v.Kind != yaml.ScalarNode {
				return "", fmt.Errorf("kesconf: invalid config version at line '%d'", v.Line)
			}
			return v.Value, nil
		}
	}
	return "", nil
}

func ymlToServerConfig(y *ymlFile) (*File, error) {
	if y.Version != "" && y.Version != "v1" {
		return nil, fmt.Errorf("kesconf: invalid config version '%s'", y.Version)
	}
	if y.Admin.Identity.Value.IsUnknown() {
		return nil, errors.New("kesconf: invalid admin identity: no admin identity")
	}
	if y.TLS.PrivateKey.Value == "" {
		return nil, errors.New("kesconf: invalid tls config: no private key")
	}
	if y.TLS.Certificate.Value == "" {
		return nil, errors.New("kesconf: invalid tls config: no certificate")
	}

	clientAuth := tls.RequireAnyClientCert
	if v := strings.ToLower(y.TLS.ClientAuth.Value); v != "" && v != "on" && v != "off" {
		return nil, fmt.Errorf("kesconf: invalid tls config: invalid auth '%s'", y.TLS.ClientAuth)
	} else if v == "on" {
		clientAuth = tls.RequireAndVerifyClientCert
	}

	for name, policy := range y.Policies {
		for _, identity := range policy.Identities {
			if identity.Value == y.Admin.Identity.Value {
				return nil, fmt.Errorf("kesconf: invalid policy '%s': identity '%s' is already admin", name, identity.Value)
			}
		}
	}

	if y.Cache.Expiry.Any.Value < 0 {
		return nil, fmt.Errorf("kesconf: invalid cache expiry '%v'", y.Cache.Expiry.Any.Value)
	}
	if y.Cache.Expiry.Unused.Value < 0 {
		return nil, fmt.Errorf("kesconf: invalid cache unused expiry '%v'", y.Cache.Expiry.Unused.Value)
	}
	if y.Cache.Expiry.Offline.Value < 0 {
		return nil, fmt.Errorf("kesconf: invalid offline cache expiry '%v'", y.Cache.Expiry.Offline.Value)
	}

	errLevel, err := parseLogLevel(y.Log.Error.Value)
	if err != nil {
		return nil, err
	}
	auditLevel, err := parseLogLevel(y.Log.Audit.Value)
	if err != nil {
		return nil, err
	}

	for path, api := range y.API.Paths {
		if api.Timeout.Value < 0 {
			return nil, fmt.Errorf("kesconf: invalid timeout '%d' for API '%s'", api.Timeout.Value, path)
		}
	}

	if len(y.Keys) > 0 {
		names := make(map[string]struct{}, len(y.Keys))
		for _, key := range y.Keys {
			if _, ok := names[key.Name.Value]; ok {
				return nil, fmt.Errorf("kesconf: invalid key config: key '%s' is defined multiple times", key.Name.Value)
			}
			names[key.Name.Value] = struct{}{}
		}
	}

	keystore, err := ymlToKeyStore(y)
	if err != nil {
		return nil, err
	}

	c := &File{
		Addr:  y.Addr.Value,
		Admin: y.Admin.Identity.Value,
		TLS: &TLSConfig{
			PrivateKey:  y.TLS.PrivateKey.Value,
			Certificate: y.TLS.Certificate.Value,
			Password:    y.TLS.Password.Value,
			ClientAuth:  clientAuth,
			CAPath:      y.TLS.CAPath.Value,
		},
		Cache: &CacheConfig{
			Expiry:        y.Cache.Expiry.Any.Value,
			ExpiryUnused:  y.Cache.Expiry.Unused.Value,
			ExpiryOffline: y.Cache.Expiry.Offline.Value,
		},
		Log: &LogConfig{
			ErrLevel:   errLevel,
			AuditLevel: auditLevel,
		},
		KeyStore: keystore,
	}
	if len(y.Policies) > 0 {
		c.Policies = make(map[string]Policy, len(y.Policies))
		for name, policy := range y.Policies {
			identities := make([]kes.Identity, 0, len(policy.Identities))
			for _, id := range policy.Identities {
				identities = append(identities, id.Value)
			}
			c.Policies[name] = Policy{
				Allow:      policy.Allow,
				Deny:       policy.Deny,
				Identities: identities,
			}
		}
	}
	if len(y.API.Paths) > 0 {
		paths := make(map[string]APIPathConfig, len(y.API.Paths))
		for path, api := range y.API.Paths {
			paths[path] = APIPathConfig{
				Timeout: api.Timeout.Value,
			}
		}
		c.API = &APIConfig{
			Paths: paths,
		}
	}
	for path, api := range y.API.Paths {
		if api.Timeout.Value < 0 {
			return nil, fmt.Errorf("kesconf: invalid timeout '%d' for API '%s'", api.Timeout.Value, path)
		}
	}
	if len(y.Keys) > 0 {
		c.Keys = make([]Key, 0, len(y.Keys))
		for _, key := range y.Keys {
			c.Keys = append(c.Keys, Key{Name: key.Name.Value})
		}
	}
	return c, nil
}

func ymlToKeyStore(y *ymlFile) (KeyStore, error) {
	var keystore KeyStore

	// Hashicorp Vault Keystore
	if y.KeyStore.Vault != nil {
		if keystore != nil {
			return nil, errors.New("kesconf: invalid keystore config: more than once keystore specified")
		}
		if y.KeyStore.Vault.Endpoint.Value == "" {
			return nil, errors.New("kesconf: invalid vault keystore: no endpoint specified")
		}
		if y.KeyStore.Vault.AppRole == nil && y.KeyStore.Vault.Kubernetes == nil {
			return nil, errors.New("kesconf: invalid vault keystore: no authentication method specified")
		}
		if y.KeyStore.Vault.AppRole != nil && y.KeyStore.Vault.Kubernetes != nil {
			return nil, errors.New("kesconf: invalid vault keystore: more than one authentication method specified")
		}
		if y.KeyStore.Vault.AppRole != nil {
			if y.KeyStore.Vault.AppRole.ID.Value == "" {
				return nil, errors.New("kesconf: invalid vault keystore: invalid approle config: no approle ID specified")
			}
			if y.KeyStore.Vault.AppRole.Secret.Value == "" {
				return nil, errors.New("kesconf: invalid vault keystore: invalid approle config: no approle secret specified")
			}
		}
		if y.KeyStore.Vault.Kubernetes != nil {
			if y.KeyStore.Vault.Kubernetes.JWT.Value == "" {
				return nil, errors.New("kesconf: invalid vault keystore: invalid kubernetes config: no JWT specified")
			}

			// If the passed JWT value contains a path separator we assume it's a file.
			// We always check for '/' and the OS-specific one make cover cases where
			// a path is specified using '/' but the underlying OS is e.g. windows.
			if jwt := y.KeyStore.Vault.Kubernetes.JWT.Value; strings.ContainsRune(jwt, '/') || strings.ContainsRune(jwt, os.PathSeparator) {
				_, err := os.ReadFile(y.KeyStore.Vault.Kubernetes.JWT.Value)
				if err != nil {
					return nil, fmt.Errorf("kesconf: failed to read vault kubernetes JWT from '%s': %v", y.KeyStore.Vault.Kubernetes.JWT.Value, err)
				}
				// postpone resolving the JWT until actually logging in
			}
		}
		if y.KeyStore.Vault.Transit != nil {
			if y.KeyStore.Vault.Transit.KeyName.Value == "" {
				return nil, errors.New("kesconf: invalid vault keystore: invalid transit config: no key name specified")
			}
		}

		if y.KeyStore.Vault.TLS.PrivateKey.Value != "" && y.KeyStore.Vault.TLS.Certificate.Value == "" {
			return nil, errors.New("kesconf: invalid vault keystore: invalid tls config: no TLS certificate provided")
		}
		if y.KeyStore.Vault.TLS.PrivateKey.Value == "" && y.KeyStore.Vault.TLS.Certificate.Value != "" {
			return nil, errors.New("kesconf: invalid vault keystore: invalid tls config: no TLS private key provided")
		}
		s := &VaultKeyStore{
			Endpoint:    y.KeyStore.Vault.Endpoint.Value,
			Namespace:   y.KeyStore.Vault.Namespace.Value,
			APIVersion:  y.KeyStore.Vault.APIVersion.Value,
			Engine:      y.KeyStore.Vault.Engine.Value,
			Prefix:      y.KeyStore.Vault.Prefix.Value,
			PrivateKey:  y.KeyStore.Vault.TLS.PrivateKey.Value,
			Certificate: y.KeyStore.Vault.TLS.Certificate.Value,
			CAPath:      y.KeyStore.Vault.TLS.CAPath.Value,
			StatusPing:  y.KeyStore.Vault.Status.Ping.Value,
		}
		if y.KeyStore.Vault.AppRole != nil {
			s.AppRole = &VaultAppRoleAuth{
				Engine:    y.KeyStore.Vault.AppRole.Engine.Value,
				Namespace: y.KeyStore.Vault.AppRole.Namespace.Value,
				ID:        y.KeyStore.Vault.AppRole.ID.Value,
				Secret:    y.KeyStore.Vault.AppRole.Secret.Value,
			}
		}
		if y.KeyStore.Vault.Kubernetes != nil {
			s.Kubernetes = &VaultKubernetesAuth{
				Engine:    y.KeyStore.Vault.Kubernetes.Engine.Value,
				Namespace: y.KeyStore.Vault.Kubernetes.Namespace.Value,
				JWT:       y.KeyStore.Vault.Kubernetes.JWT.Value,
				Role:      y.KeyStore.Vault.Kubernetes.Role.Value,
			}
		}
		if y.KeyStore.Vault.Transit != nil {
			s.Transit = &VaultTransit{
				Engine:  y.KeyStore.Vault.Transit.Engine.Value,
				KeyName: y.KeyStore.Vault.Transit.KeyName.Value,
			}
		}
		keystore = s
	}

	if keystore == nil {
		return nil, errors.New("kesconf: no keystore specified")
	}
	return keystore, nil
}

type env[T any] struct {
	Var   string
	Value T
}

func (r env[T]) MarshalYAML() (any, error) {
	if env := strings.TrimSpace(r.Var); env != "" {
		switch p, s := strings.HasPrefix(env, "${"), strings.HasSuffix(env, "}"); {
		case p && s:
			return env, nil
		case !p && !s:
			return "${" + env + "}", nil
		default:
			return nil, fmt.Errorf("kesconf: invalid env. variable reference '%s'", r.Var)
		}
	}
	return r.Value, nil
}

func (r *env[T]) UnmarshalYAML(node *yaml.Node) error {
	var env string
	if v := strings.TrimSpace(node.Value); strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		env = strings.TrimSpace(v[2 : len(v)-1])
		v, ok := os.LookupEnv(env)
		if !ok {
			return fmt.Errorf("kesconf: referenced env. variable '%s' in line '%d' not found", env, node.Line)
		}
		node.Value = v
	}

	var v T
	if err := node.Decode(&v); err != nil {
		return err
	}
	r.Var = env
	r.Value = v
	return nil
}

func parseLogLevel(s string) (slog.Level, error) {
	const (
		LevelDebug = "DEBUG"
		LevelInfo  = "INFO"
		LevelWarn  = "WARN"
		LevelError = "ERROR"

		// Pseudo-levels for backward compatibility.
		LevelOn  = "ON"  // Equal to LevelInfo
		LevelOff = "OFF" // Equal to LevelError+1
	)
	if s = strings.TrimSpace(strings.ToUpper(s)); s == "" {
		return slog.LevelInfo, nil
	}
	if s == LevelOn {
		return slog.LevelInfo, nil
	}
	if s == LevelOff {
		return slog.LevelError + 1, nil
	}

	parseLevel := func(val string, base slog.Level) (slog.Level, error) {
		level, suffix, ok := strings.Cut(val, "+")
		if !ok || strings.TrimSpace(level) != base.String() {
			return 0, fmt.Errorf("kesconf: invalid log level '%s'", val)
		}

		n, err := strconv.Atoi(suffix)
		if err != nil {
			return 0, fmt.Errorf("kesconf: invalid log level suffix '%s': %v", suffix, err)
		}
		return base + slog.Level(n), nil
	}

	switch {
	case strings.HasPrefix(s, LevelDebug):
		if s == LevelDebug {
			return slog.LevelDebug, nil
		}
		return parseLevel(s, slog.LevelDebug)
	case strings.HasPrefix(s, LevelInfo):
		if s == LevelInfo {
			return slog.LevelInfo, nil
		}
		return parseLevel(s, slog.LevelInfo)
	case strings.HasPrefix(s, LevelWarn):
		if s == LevelWarn {
			return slog.LevelWarn, nil
		}
		return parseLevel(s, slog.LevelWarn)
	case strings.HasPrefix(s, LevelError):
		if s == LevelError {
			return slog.LevelError, nil
		}
		return parseLevel(s, slog.LevelError)
	default:
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		return slog.Level(n), nil
	}
}
