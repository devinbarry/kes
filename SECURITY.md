# Security policy

This repository is an independently maintained, hardened fork of [MinIO KES](https://github.com/minio/kes). MinIO
archived `minio/kes` in June 2025 and does not handle reports for it. Report security problems in this fork here,
not to MinIO or to other forks.

## Reporting a vulnerability

Report suspected vulnerabilities privately through GitHub: open this repository's **Security** tab and choose
**Report a vulnerability**. The report is visible only to the maintainer of this fork.

Do not open a public issue, pull request or discussion about a security problem.

Where you can, include:

- the affected release or commit;
- the relevant configuration, with secrets, identities and addresses removed;
- the steps to reproduce the problem and its impact.

## Supported versions

Only the latest release receives security fixes.

## Scope

In scope: the KES server and CLI in this repository, including the Vault key store, configuration parsing and the
TLS and authentication policy. Vulnerabilities in HashiCorp Vault, MinIO or a dependency belong with their own
maintainers; a dependency issue is in scope here when this code makes it reachable.
