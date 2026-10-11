# KES (hardened fork)

A security-focused fork of [MinIO KES](https://github.com/minio/kes), the key encryption server that sits between
MinIO and a key store and manages the keys MinIO uses for server-side encryption. MinIO archived KES in June 2025.

This fork continues from [kypello-io/kes](https://github.com/kypello-io/kes), a community fork that carries all of
upstream `minio/kes` plus its own Go, AWS SDK and lint updates. It is based on kypello-io/kes at commit
[`d935da4`](https://github.com/kypello-io/kes/commit/d935da49014a881ea87ac8c630f6aeb17cfebcda).

The fork is deliberately narrow. It supports one deployment shape, KES in front of HashiCorp Vault with mutual TLS
on every connection, and removes everything else. If you need another key store or a less strict mode, use
upstream KES or kypello-io/kes instead.

This project is not affiliated with or endorsed by MinIO, Inc.

## What differs from upstream

- **Vault is the only key store,** over `https://` only, with the K/V v2 engine and AppRole login. The AWS,
  Azure, GCP, Fortanix, Gemalto, Entrust and filesystem stores, K/V v1 and Kubernetes login are removed.
- **TLS 1.3 everywhere.** The server, the Vault client and the CLI refuse anything older.
- **Every connection needs a client certificate.** The per-route `skip_auth` option and the TLS-proxy option are
  removed.
- **Vault trust comes from the config file only.** KES refuses to start if any `VAULT_*` variable is set, never
  uses an HTTP proxy for Vault, and fails on an unreadable CA file.
- **A configured CA replaces the system roots** rather than adding to them.
- **Strict configuration.** Unknown or removed keys fail startup.
- **No insecure or demo modes.** `kes server --dev`, `kes update`, `kes migrate`, the CLI's `-k/--insecure`
  and `--color` options are gone. The CLI verifies the server against `MINIO_KES_CA_FILE` when it is set.
- **Current dependencies.** Every module is kept on its newest stable release once that release is at least 48
  hours old.

These changes break compatibility with upstream configurations that use any removed option.

## Building

KES needs Go 1.27 or later. The Go module path is still `github.com/minio/kes`, so
`go install github.com/minio/kes/...` fetches the archived upstream, not this fork. Build from a clone instead:

```sh
CGO_ENABLED=0 go build -trimpath -o kes ./cmd/kes
./kes --version
```

The test suite runs offline:

```sh
go test ./...
```

### Container image

The `Dockerfile` runs `gofmt`, `go vet` and the tests, then builds a static binary into a `scratch` image. Its
default builder image is a private mirror, so pass a Go image yourself, ideally pinned by digest:

```sh
docker build \
  --build-arg GO_IMAGE=golang:1.27.1-alpine3.24 \
  --build-arg SOURCE_COMMIT="$(git rev-parse HEAD)" \
  -t kes .
```

The image contains only `/kes` and the licence. It runs as `65532:65532`, listens on port 7373 and starts
`kes server --config /etc/kes/config.yaml`. It has **no system CA roots**: KES trusts only the CA files named in
its configuration, so mount them alongside the config, certificates and keys.

## Configuration

[`server-config.yaml`](server-config.yaml) documents every option. A minimal configuration for MinIO looks like
this:

```yaml
address: 0.0.0.0:7373

tls:
  cert: /etc/kes/server.crt
  key: /etc/kes/server.key

admin:
  identity: disabled            # no admin identity; access is granted only through policies

policy:
  minio:
    allow:
      - /v1/key/create/*
      - /v1/key/generate/*
      - /v1/key/decrypt/*
      - /v1/key/bulk/decrypt/*
      - /v1/key/list/*
      - /v1/status
      - /v1/metrics
      - /v1/log/audit
      - /v1/log/error
    identities:
      - ${MINIO_IDENTITY}       # environment variables are expanded

keystore:
  vault:
    endpoint: https://vault.example.invalid:8200
    engine: kv
    version: v2
    prefix: kes
    transit:
      engine: transit
      key: kes
    approle:
      id: ${KES_APPROLE_ID}
      secret: ${KES_APPROLE_SECRET}
    tls:
      ca: /etc/kes/vault-ca.crt
```

Points that differ from upstream:

- Clients are identified by the hash of their certificate's public key (`kes identity of <cert>`). With `tls.auth`
  unset or `off`, any client certificate completes the handshake and policies decide what it may do. With `on`,
  the certificate must also chain to `tls.ca`. A client without a certificate is always rejected.
- `keystore.vault.version` must be `v2` and `endpoint` must be `https://`.
- Do not name configuration variables `VAULT_*` in the server's own environment: KES refuses to start if any
  `VAULT_*` variable is set. Use other names, as above.

## CLI

The `kes` client commands (`key`, `policy`, `identity`, `status`, `log`, `metric`) read these variables:

| Variable | Meaning |
| --- | --- |
| `MINIO_KES_SERVER` | Server endpoint(s), comma-separated. Defaults to `127.0.0.1:7373`. |
| `MINIO_KES_CERT_FILE`, `MINIO_KES_KEY_FILE` | Client certificate and private key. |
| `MINIO_KES_API_KEY` | An API key, as an alternative to a certificate and key. |
| `MINIO_KES_CA_FILE` | CA certificates used to verify the server. If unset, the system roots are used. |

There is no option to skip server certificate verification.

## Security

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). Please do not open public issues for
security problems.

## Licence

KES is licensed under the GNU Affero General Public License v3.0; see [`LICENSE`](LICENSE). This is a modified
version of MinIO KES: kypello-io/kes changed it until March 2026, and this fork has changed it since October
2026. The commit history records each change. Copyright in the upstream code remains with MinIO, Inc. and its
contributors, and the changes in this fork are released under the same licence.
