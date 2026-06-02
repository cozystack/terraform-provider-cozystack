# Contributing

Thank you for your interest in contributing to the Cozystack Terraform provider!

## Code of Conduct

Be respectful and constructive in all interactions. Contributors of all experience levels are welcome.

## How to Contribute

### Reporting Issues

- Search existing issues before creating a new one.
- Use the issue templates for bug reports and feature requests.
- Provide as much detail as possible (provider/Terraform/Cozystack versions, configuration, plan output). Redact tokens and private infrastructure details.

### Submitting Pull Requests

1. Fork the repository.
2. Create a feature branch from `master`.
3. Make your changes (add or update tests and examples).
4. Ensure tests, linting, and docs generation are clean.
5. Submit a pull request and fill out the template.

## Development Setup

```bash
git clone https://github.com/lexfrei/terraform-provider-cozystack.git
cd terraform-provider-cozystack
make build   # build the provider binary
make test    # run unit tests with the race detector
make lint    # run golangci-lint
make docs    # regenerate registry docs from schema + examples
```

Run `make help` to see all available targets.

### Local Testing with `dev_overrides`

To exercise the provider with a real `tofu`/`terraform` configuration before it is published, install it into your `GOBIN` and point a CLI config at it:

```bash
make install
```

Then add a `dev_overrides` block to `~/.terraformrc` (or a `TF_CLI_CONFIG_FILE`) mapping `registry.opentofu.org/lexfrei/cozystack` to your `GOBIN`. With an override in place, run `tofu plan` directly — no `tofu init` is needed.

### Acceptance Tests

Acceptance tests run real CRUD against a live Cozystack cluster and are gated behind `TF_ACC`:

```bash
KUBECONFIG=/path/to/kubeconfig KUBE_CTX=your-context make testacc
```

These create and destroy real objects. Point them at a disposable test cluster, never production. The cluster name is never hard-coded — it is read from the `KUBECONFIG`/`KUBE_CTX` environment.

## Adding a Resource or Data Source

Most kinds are wired through the generic `appResource` / `newClusterResource` factories. When adding a kind:

1. Add the `client.Resource` constructor (GVR, scope) under `internal/client/`.
2. Register the resource and data source in `internal/provider/provider.go`.
3. Add a typed model only when the spec benefits from it; otherwise use the raw-JSON spec passthrough.
4. Add a unit test, including the reflection spec-coverage guard where a typed model is used.
5. Add `examples/resources/` and `examples/data-sources/` entries, then run `make docs`.

## Commit Message Format

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```text
type(scope): brief description

Optional longer explanation.
```

### Types

| Type       | Description                            |
| ---------- | -------------------------------------- |
| `feat`     | New feature                            |
| `fix`      | Bug fix                                |
| `docs`     | Documentation changes                  |
| `refactor` | Code refactoring                       |
| `test`     | Adding or updating tests               |
| `chore`    | Maintenance tasks                      |
| `ci`       | CI/CD changes                          |

### Examples

```text
feat(provider): add cozystack_backup_strategy resource

fix(client): retry update on resource-version conflict

docs(examples): add a kubernetes tenant example
```

## Code Style

- Follow standard Go conventions; format with `make fmt`.
- Lint with `golangci-lint` (config in `.golangci.yaml`); `make lint` must pass with zero errors.
- If you must disable a linter, add a `//nolint:<linter> // reason` directive with a justification.
- Add godoc comments to exported types and functions.

## Documentation

- Registry docs under `docs/` are generated — do not edit them by hand. Edit the schema descriptions or `examples/`, then run `make docs`.
- A pull request must not leave `make docs` producing a diff.

## Release Process

Releases are tag-driven and automated via GoReleaser in GitHub Actions: pushing a signed `vX.Y.Z` tag builds the multi-platform archives, signs the checksums, and publishes a GitHub Release that the Terraform Registry picks up. The provider version tracks the Cozystack API version it is built against.

## License

By contributing, you agree that your contributions will be licensed under the BSD 3-Clause License.
