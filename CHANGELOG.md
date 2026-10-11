# Changelog

This fork's releases. Earlier versions are those of [kypello-io/kes](https://github.com/kypello-io/kes) and
[minio/kes](https://github.com/minio/kes).

## v2.0.0 - 2026-10-11

The first release of this fork, based on kypello-io/kes at
[`d935da4`](https://github.com/kypello-io/kes/commit/d935da49014a881ea87ac8c630f6aeb17cfebcda). It breaks
compatibility with upstream configurations and CLI usage.

### Removed

- Every key store except HashiCorp Vault: AWS Secrets Manager, Azure Key Vault, GCP Secret Manager, Fortanix SDKMS,
  Gemalto KeySecure, Entrust KeyControl, and the filesystem and encrypted-filesystem stores.
- Vault K/V v1 and Vault Kubernetes login.
- The per-API `skip_auth` option and the TLS-proxy (`tls.proxy`) option.
- `kes server --dev`, `kes update`, `kes migrate`, the server's deprecated `--key`, `--cert` and `--auth` flags,
  and the CLI's `-k/--insecure` and `--color` options.

### Changed

- TLS 1.3 is the minimum for the server, the Vault client and the CLI.
- Every connection must present a client certificate.
- The Vault endpoint must be `https://`, the K/V engine must be v2 and AppRole is the only login method.
- KES refuses to start if any `VAULT_*` environment variable is set, and never routes Vault traffic through an HTTP
  proxy.
- A configured CA file replaces the system roots instead of adding to them, and an unreadable CA file is an error.
- Unknown or removed configuration keys fail startup.
- The CLI verifies the server against `MINIO_KES_CA_FILE` when it is set.
- All Go modules are upgraded to their newest stable releases, and the build uses Go 1.27.
- The container image is built `FROM scratch` with a static binary that runs as `65532:65532`.

### Added

- `SECURITY.md`, with private vulnerability reporting for this fork.
- A test that keeps `server-config.yaml` in line with the options this fork accepts.

### Notes

- Build with Go 1.27.2 or later: it fixes three standard-library denial-of-service advisories (CVE-2026-78667,
  CVE-2026-78669, CVE-2026-97031) that affect binaries built with Go 1.27.1.
