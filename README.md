# KES (hardened fork)

A private, security-maintained fork of [MinIO KES](https://github.com/minio/kes), the key encryption server
between our MinIO deployment and HashiCorp Vault. MinIO archived KES in June 2025 and no maintained
open-source build exists, so this fork keeps it patched for our own use only.

## What differs from upstream

The fork follows our deployment, not upstream compatibility:

- **Vault is the only key store,** over `https://` only, with the K/V v2 engine and AppRole login. The AWS,
  Azure, GCP, Fortanix, Gemalto, Entrust and filesystem stores, K/V v1 and Kubernetes login are removed.
- **TLS 1.3 everywhere.** The server, the Vault client and the CLI refuse anything older.
- **Every connection needs a client certificate.** The per-route `skip_auth` option and the unwired TLS-proxy
  option are removed.
- **Vault trust comes from the config file only.** KES refuses to start if any `VAULT_*` variable is set, never
  uses an HTTP proxy for Vault, and fails on an unreadable CA file.
- **A configured CA replaces the system roots** rather than adding to them.
- **Strict configuration.** Unknown or removed keys fail startup.
- **No insecure or demo modes.** `kes server --dev`, `kes update`, `kes migrate`, the CLI's `-k/--insecure`
  and `--color` options are gone. The CLI verifies the server against `MINIO_KES_CA_FILE` when it is set.
- **Current dependencies.** Every module is on its newest release that has cleared a 48-hour
  quarantine.

The base is [kypello-io/kes](https://github.com/kypello-io/kes) at `d935da4`, which contains all of upstream
`minio/kes` plus its own Go, AWS SDK and lint updates.

## Image

CI publishes `<registry>/ops/kes:<short-sha>` from `master`. The image is `scratch` with a static binary
at `/kes`, runs as `65532:65532` by default and contains no system CA roots. Consumers pin the exact digest
recorded in the build job's `IMAGE_REF` artifact.

## Licence

AGPLv3, as upstream. See `LICENSE`.
