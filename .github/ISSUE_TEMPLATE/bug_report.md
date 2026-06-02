---
name: Bug Report
about: Report a bug or unexpected behavior
title: '[BUG] '
labels: kind/bug, triage/needs-triage
assignees: ''
---

## Description

A clear and concise description of what the bug is.

## Steps to Reproduce

1. Provider/resource configuration '...'
2. Run `tofu plan` / `tofu apply`
3. See error

## Expected Behavior

What you expected to happen.

## Actual Behavior

What actually happened.

## Environment

- **Provider Version**: [e.g., v1.4.2]
- **Terraform/OpenTofu Version**: [e.g., OpenTofu 1.9.0]
- **Cozystack Version**: [e.g., 1.4.2]
- **Kubernetes Version**: [e.g., v1.31.0]

## Configuration

<details>
<summary>Terraform configuration</summary>

```hcl
Paste the minimal provider + resource configuration that reproduces the issue.
Redact tokens, hosts, and any private infrastructure details.
```

</details>

## Output

<details>
<summary>plan / apply output</summary>

```text
Paste the relevant `tofu plan` / `tofu apply` output. Run with TF_LOG=DEBUG
for more detail; redact secrets before pasting.
```

</details>

## Additional Context

Add any other context about the problem here.
