# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- `make build` — build the provider binary; `make install` — `go install` into `GOBIN` for `~/.terraformrc` `dev_overrides`.
- `make test` — unit tests with the race detector. Single test: `go test -race -run '^TestName$' ./internal/provider/`.
- `make testacc` — acceptance tests; they create and destroy real objects and need a live Cozystack cluster via `KUBECONFIG`/`KUBE_CTX` (gated by `TF_ACC=1`, points `TF_ACC_TERRAFORM_PATH` at `tofu`/`terraform` automatically). Single acceptance test: `KUBECONFIG=... KUBE_CTX=... TF_ACC=1 go test -run '^TestAccName$' ./internal/provider/`.
- `make lint` (`golangci-lint`) and `make fmt` must be clean before committing. CI runs a possibly stricter `golangci-lint` — `goconst` fires at 2+ occurrences there even when local says clean.
- `make docs` — regenerate `docs/` from the schema + `examples/` via `tfplugindocs`. It builds the provider and exports the schema through a `tofu` dev-override (`scripts/gen-docs.sh`); a PR must not leave `make docs` producing a diff. Never hand-edit `docs/`.

## Architecture

The provider talks to the Cozystack aggregated Kubernetes API (`apps.cozystack.io/v1alpha1` and sibling groups) with `client-go`'s **dynamic client** — there is no typed clientset. "Typing" lives at the Terraform layer: typed schema attributes ↔ `map[string]any` spec ↔ `unstructured` over the wire. Most Kinds are an `Application` whose `spec` is an opaque JSON blob the server turns into a FluxCD `HelmRelease`.

- `internal/client/` — the wire layer. `application.go` holds the generalized `client.Resource{Resource,Kind,Group,Version,ClusterScoped,NoSpec}` (defaults to `apps.cozystack.io`/`v1alpha1`/namespaced) and CRUD over the dynamic client. `config.go` builds the `rest.Config` (kubeconfig/context/`host`+`token`/mTLS/`in_cluster`/`exec` credential plugin). `outputs.go` reads server-generated child Secrets/Services for connection details. `platform.go`/`groups.go` declare the `cozystack.io`, `backups.cozystack.io`, and `core.cozystack.io` Resources.
- `internal/provider/` — the Terraform layer. `provider.go` registers every resource/data source/ephemeral. `generic_resource.go` + `crud.go` are the engine: `newAppResource`/`newAppDataSource` (namespaced) and `newClusterResource` (cluster-scoped, imports by name) wire any Kind that supplies an `expand`/`flatten` model. Each Kind contributes a `*_model.go` (pure `expand`/`flatten`, no k8s) and `*_schema.go`.
- Three model styles: **typed** (`tenant`, `postgresql`, `backups_typed`, …) for specs worth modelling; **rawspec** (`rawspec_model.go`) for `spec = jsonencode(...)` passthrough using `jsontypes.Normalized`; **marker** (`marker_model.go`) for spec-less Kinds (`NoSpec`). `tenant_secret.go` is bespoke (data/`stringData`, not `.spec`) with a write-only `data_wo` attribute; `ephemeral.go` exposes kubeconfig/tenant-secret as ephemeral resources to keep secrets out of state. `outputsReader` (`crud.go`) surfaces `kubeconfig`/`endpoints`/`credentials`/`ip_address`; `uid` is exposed on every Kind.

## Conventions

- The provider version tracks the Cozystack API version it builds against; the `github.com/cozystack/cozystack/api/apps/v1alpha1` module is pinned to a released tag (currently `v1.4.2`), never a pseudo-version.
- Typed models are guarded against upstream spec drift by `assertSpecCoverage(t, emitted, ConfigSpec{}, omit...)`, which reflects emitted spec keys against the api-module's `ConfigSpec` json tags. For Kinds whose Go types live in the heavy root cozystack module, a small vendored guard struct stands in.
- Adding a Kind: add the `client.Resource` constructor, register it in `provider.go`, choose typed vs rawspec vs marker, add a unit test (with the spec-coverage guard for typed models), add `examples/resources/` + `examples/data-sources/` entries, then run `make docs`.

## Releases

Releases are cut **manually and locally** with GoReleaser (`goreleaser release --clean`) and signed with the maintainer GPG key (`9C173EB1B531AA1F`, forced via the trailing `!` in `.goreleaser.yaml`). There is intentionally no release CI workflow; only `ci.yaml` (test/lint/markdown/CodeQL) runs in GitHub Actions.
