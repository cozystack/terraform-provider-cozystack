# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, report vulnerabilities via email:

- **Email**: <f@lex.la>
- **GPG Key**: `F57F 85FC 7975 F22B BC3F 2504 9C17 3EB1 B531 AA1F`

### What to Include

- Type of vulnerability
- Full paths of affected source files
- Location of affected source code (tag/branch/commit)
- Step-by-step reproduction instructions
- Proof-of-concept or exploit code (if possible)
- Impact assessment

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Timeline**: Depends on severity

## Security Best Practices

### Credentials

The provider authenticates to the Cozystack aggregated API the same way the official `kubernetes` provider does. Treat every credential as sensitive:

- Prefer a kubeconfig context or an `exec {}` credential plugin (OIDC login helper) over a static bearer token committed to configuration.
- Never hard-code `token`, `client_key`, or kubeconfig contents in `.tf` files. Source them from environment variables, a secrets manager, or a Terraform variable marked `sensitive`.
- Scope the Kubernetes identity to the tenants and namespaces the configuration actually manages, not cluster-admin.

### Secrets in State

Terraform state is plaintext by default. To keep server-generated and user-supplied secrets out of state:

- Use **ephemeral resources** (`cozystack_kubernetes` / `cozystack_tenant_secret` ephemeral variants) to read kubeconfigs and secret data without persisting them.
- Use the **write-only** `data_wo` attribute on `cozystack_tenant_secret` so secret material is sent to the API but never stored in state.
- Store the state backend itself encrypted at rest and restrict access to it.

### Secrets in Logs

The provider does not log credential values. Running with `TF_LOG=DEBUG` may surface request bodies — redact any output before sharing it in an issue.

## Dependency and Supply-Chain Security

- Dependencies are kept current via Renovate.
- Releases are signed: the published checksums are GPG-signed, and the GitHub Actions build provenance is attached to each release.
- Verify a downloaded release against the signed `SHA256SUMS` before use.
