# terraform-provider-cozystack

A Terraform / OpenTofu provider for [Cozystack](https://cozystack.io). It manages Cozystack applications through the aggregated Kubernetes API (`apps.cozystack.io`), so a Cozystack platform can be driven as Infrastructure as Code with the same kubeconfig you already use.

## Supported resources

Every kind is served by the same aggregated API, so the provider is built to grow one kind at a time. Adding a kind is a typed model, a schema, and a one-line resource descriptor — the create/read/update/delete/import/wait logic is shared.

| Kind | Resource | Data source |
| --- | --- | --- |
| Tenant — isolated namespace under a parent tenant | [`cozystack_tenant`](docs/resources/tenant.md) | [`cozystack_tenant`](docs/data-sources/tenant.md) |
| Managed Redis | [`cozystack_redis`](docs/resources/redis.md) | [`cozystack_redis`](docs/data-sources/redis.md) |
| Managed Qdrant (vector database) | [`cozystack_qdrant`](docs/resources/qdrant.md) | [`cozystack_qdrant`](docs/data-sources/qdrant.md) |
| S3-compatible bucket | [`cozystack_bucket`](docs/resources/bucket.md) | [`cozystack_bucket`](docs/data-sources/bucket.md) |
| Managed OpenBAO (Vault-compatible) | [`cozystack_openbao`](docs/resources/openbao.md) | [`cozystack_openbao`](docs/data-sources/openbao.md) |
| VPN server | [`cozystack_vpn`](docs/resources/vpn.md) | [`cozystack_vpn`](docs/data-sources/vpn.md) |
| Managed RabbitMQ | [`cozystack_rabbitmq`](docs/resources/rabbitmq.md) | [`cozystack_rabbitmq`](docs/data-sources/rabbitmq.md) |
| Managed MariaDB | [`cozystack_mariadb`](docs/resources/mariadb.md) | [`cozystack_mariadb`](docs/data-sources/mariadb.md) |
| Managed MongoDB | [`cozystack_mongodb`](docs/resources/mongodb.md) | [`cozystack_mongodb`](docs/data-sources/mongodb.md) |
| Managed ClickHouse | [`cozystack_clickhouse`](docs/resources/clickhouse.md) | [`cozystack_clickhouse`](docs/data-sources/clickhouse.md) |
| Managed NATS | [`cozystack_nats`](docs/resources/nats.md) | [`cozystack_nats`](docs/data-sources/nats.md) |
| Managed OpenSearch | [`cozystack_opensearch`](docs/resources/opensearch.md) | [`cozystack_opensearch`](docs/data-sources/opensearch.md) |
| Managed PostgreSQL | [`cozystack_postgres`](docs/resources/postgres.md) | [`cozystack_postgres`](docs/data-sources/postgres.md) |
| HTTP cache | [`cozystack_httpcache`](docs/resources/httpcache.md) | [`cozystack_httpcache`](docs/data-sources/httpcache.md) |
| TCP load balancer | [`cozystack_tcpbalancer`](docs/resources/tcpbalancer.md) | [`cozystack_tcpbalancer`](docs/data-sources/tcpbalancer.md) |
| Harbor registry | [`cozystack_harbor`](docs/resources/harbor.md) | [`cozystack_harbor`](docs/data-sources/harbor.md) |
| Virtual private cloud | [`cozystack_vpc`](docs/resources/vpc.md) | [`cozystack_vpc`](docs/data-sources/vpc.md) |
| Virtual machine disk | [`cozystack_vmdisk`](docs/resources/vmdisk.md) | [`cozystack_vmdisk`](docs/data-sources/vmdisk.md) |
| Managed Kafka | [`cozystack_kafka`](docs/resources/kafka.md) | [`cozystack_kafka`](docs/data-sources/kafka.md) |
| Managed FoundationDB | [`cozystack_foundationdb`](docs/resources/foundationdb.md) | [`cozystack_foundationdb`](docs/data-sources/foundationdb.md) |
| Virtual machine instance | [`cozystack_vminstance`](docs/resources/vminstance.md) | [`cozystack_vminstance`](docs/data-sources/vminstance.md) |
| Managed Kubernetes | [`cozystack_kubernetes`](docs/resources/kubernetes.md) | [`cozystack_kubernetes`](docs/data-sources/kubernetes.md) |

Every kind served by the aggregated `apps.cozystack.io` API is now covered.

## Platform resources (`cozystack.io` group)

Beyond the namespaced tenant apps, the provider also manages the cluster-scoped platform resources in the `cozystack.io` group — useful for platform-as-code:

| Kind | Resource | Data source |
| --- | --- | --- |
| Package — install a package variant | [`cozystack_package`](docs/resources/package.md) | [`cozystack_package`](docs/data-sources/package.md) |
| PackageSource — where packages come from | [`cozystack_package_source`](docs/resources/package_source.md) | [`cozystack_package_source`](docs/data-sources/package_source.md) |
| ApplicationDefinition — register an app kind | [`cozystack_application_definition`](docs/resources/application_definition.md) | [`cozystack_application_definition`](docs/data-sources/application_definition.md) |
| SchedulingClass — named placement policy | [`cozystack_scheduling_class`](docs/resources/scheduling_class.md) | [`cozystack_scheduling_class`](docs/data-sources/scheduling_class.md) |

These are cluster-scoped (imported by name, no namespace). `cozystack_package` is fully typed (`variant`, `ignore_dependencies`, per-component overrides). The other three are platform-definition documents whose deeply-nested, version-coupled specs are surfaced as a single normalized-JSON `spec` attribute (write with `jsonencode`, read back with `jsondecode`) rather than brittle per-field modeling.

### Backups and dashboard

The same JSON-spec passthrough also covers the backup framework and dashboard panels:

| Kind | Resource | Data source |
| --- | --- | --- |
| BackupClass | [`cozystack_backup_class`](docs/resources/backup_class.md) | [`cozystack_backup_class`](docs/data-sources/backup_class.md) |
| Backup | [`cozystack_backup`](docs/resources/backup.md) | [`cozystack_backup`](docs/data-sources/backup.md) |
| BackupJob | [`cozystack_backup_job`](docs/resources/backup_job.md) | [`cozystack_backup_job`](docs/data-sources/backup_job.md) |
| Plan | [`cozystack_backup_plan`](docs/resources/backup_plan.md) | [`cozystack_backup_plan`](docs/data-sources/backup_plan.md) |
| RestoreJob | [`cozystack_restore_job`](docs/resources/restore_job.md) | [`cozystack_restore_job`](docs/data-sources/restore_job.md) |
| MarketplacePanel | [`cozystack_marketplace_panel`](docs/resources/marketplace_panel.md) | [`cozystack_marketplace_panel`](docs/data-sources/marketplace_panel.md) |

### Tenant core resources (`core.cozystack.io`)

These do not follow the spec pattern, so they have purpose-built models:

| Kind | Resource | Shape |
| --- | --- | --- |
| TenantSecret | [`cozystack_tenant_secret`](docs/resources/tenant_secret.md) | Secret-shaped: `type` + `data` (plaintext in, stored base64) |
| TenantModule | [`cozystack_tenant_module`](docs/resources/tenant_module.md) | namespaced marker (existence enables the module) |
| TenantNamespace | [`cozystack_tenant_namespace`](docs/resources/tenant_namespace.md) | cluster-scoped marker |

Deliberately not exposed: the `strategy.backups.cozystack.io` per-engine backup strategies and the `dashboard.cozystack.io` UI-customization CRDs (Sidebar, Navigation, Factory, …) — platform internals shipped and reconciled by Cozystack itself, with no Infrastructure-as-Code use case.

Authentication mirrors the official kubernetes provider: `config_path`/`config_context`, `host`/`token`, `cluster_ca_certificate`, `client_certificate`/`client_key`, an `in_cluster` toggle, and an `exec {}` credential-plugin block for OIDC login helpers (`kubectl oidc-login`). OIDC via a kubeconfig already works through `config_path` with no extra configuration.

## Referencing outputs

The aggregated API is write-oriented: it takes a spec and returns a thin status. The connection details you actually want to reference — endpoints, credentials, a child cluster's kubeconfig, a VM's IP — are materialised by the underlying charts as Secrets, Services, and KubeVirt status. The provider reads those and exposes them as computed (sensitive where appropriate) attributes:

- `cozystack_kubernetes.<x>.kubeconfig` — admin kubeconfig of the provisioned cluster, for chaining the `kubernetes`/`helm` providers into it.
- `cozystack_postgres.<x>.endpoints` — `{ host, read_host, port }` from the CNPG `rw`/`ro` Services.
- `cozystack_bucket.<x>.credentials["<user>"]` — `{ endpoint, bucket_name, region, access_key, secret_key }` per user.
- `cozystack_vminstance.<x>.ip_address` / `.ip_addresses` — guest addresses from the backing VirtualMachineInstance.

These are populated asynchronously, after the application is ready. Set `wait_for_ready = true` on the resource you consume so they are available on first apply rather than on a later refresh.

### Keeping secrets out of state

To avoid persisting secrets in state, the provider offers modern Terraform secret handling:

- **Ephemeral resources** (`ephemeral "cozystack_kubernetes"`, `ephemeral "cozystack_tenant_secret"`) fetch a cluster kubeconfig or a tenant secret at apply time, usable to configure downstream providers, without ever writing the value to state.
- **Write-only inputs** — `cozystack_tenant_secret` accepts `data_wo` (write-only, sent on apply but never stored; bump `data_wo_version` to push changes) alongside the state-persisted `data`.

```hcl
resource "cozystack_kubernetes" "app" {
  name           = "app"
  namespace      = "tenant-root"
  node_groups    = { md0 = { min_replicas = 1, max_replicas = 3 } }
  wait_for_ready = true
}

provider "kubernetes" {
  alias = "app"
  # chain straight into the cluster this provider just created
  # (parse cozystack_kubernetes.app.kubeconfig with the helm/kubernetes provider's
  #  config_path written from it, or your preferred kubeconfig wiring)
}
```

The names of these backing Secrets/Services are chart conventions (`postgres-<name>-rw`, `bucket-<name>-<user>`, `kubernetes-<name>-admin-kubeconfig`), pinned to the supported Cozystack version — not part of the stable aggregated-API contract.

## Requirements

- Terraform >= 1.0 or OpenTofu >= 1.6
- A running Cozystack cluster and a kubeconfig (or bearer token) that can reach its API

## Usage

```terraform
terraform {
  required_providers {
    cozystack = {
      source = "lexfrei/cozystack"
    }
  }
}

provider "cozystack" {
  config_path    = "~/.kube/config"
  config_context = "my-cozystack-cluster"
}

resource "cozystack_tenant" "team_a" {
  name      = "team-a"
  namespace = "tenant-root"

  monitoring = true
  ingress    = true
}

resource "cozystack_redis" "cache" {
  name      = "cache"
  namespace = cozystack_tenant.team_a.status_namespace

  replicas = 2
  version  = "v8"
}
```

Resources are namespaced by tenant, so other applications reference a tenant's `status_namespace` to deploy inside it.

## Authentication

Connection settings mirror the official `kubernetes` provider, so existing kubeconfig and environment conventions transfer directly. Every provider attribute is optional and falls back to a `KUBE_*` environment variable: `config_path` (`KUBE_CONFIG_PATH`, then `KUBECONFIG`), `config_context` (`KUBE_CTX`), `host` (`KUBE_HOST`), `token` (`KUBE_TOKEN`), `cluster_ca_certificate` (`KUBE_CLUSTER_CA_CERT_DATA`), and `insecure` (`KUBE_INSECURE`). Set `in_cluster = true` to use a pod's service account instead of a kubeconfig.

## Local install

Until a release is published to the registry, build the provider and point Terraform/OpenTofu at it with a dev override:

```sh
make install   # go install into $GOBIN
```

```hcl
# ~/.terraformrc  (or a file referenced by TF_CLI_CONFIG_FILE)
provider_installation {
  dev_overrides {
    "registry.terraform.io/lexfrei/cozystack" = "/path/to/your/gobin"
  }
  direct {}
}
```

## Development

```sh
make build   # build the provider binary
make test    # unit tests with the race detector
make lint    # golangci-lint
make docs    # regenerate docs/ with tfplugindocs
```

Acceptance tests create and destroy real applications and need a reachable Cozystack cluster:

```sh
KUBECONFIG=/path/to/kubeconfig KUBE_CTX=my-context make testacc
```

The acceptance harness drives the Terraform CLI; the `testacc` target points `TF_ACC_TERRAFORM_PATH` at `tofu` automatically so it runs against OpenTofu when Terraform is not installed.

## License

[BSD-3-Clause](LICENSE)
