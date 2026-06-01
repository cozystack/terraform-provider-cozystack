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

More kinds (managed Kubernetes, virtual machines, …) follow the same pattern.

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
