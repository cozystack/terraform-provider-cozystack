# terraform-provider-cozystack

A Terraform/OpenTofu provider for [Cozystack](https://cozystack.io). It manages Cozystack applications through the aggregated Kubernetes API (`apps.cozystack.io`), so a Cozystack platform can be driven as Infrastructure as Code.

## Status

It currently manages **tenants** (the isolated namespaces under a parent tenant in which other Cozystack applications run) and **Redis** instances. Every kind is served by the same aggregated API, so more kinds (Postgres, Kubernetes, virtual machines, and so on) follow the same pattern and can be added incrementally.

- Resources: [`cozystack_tenant`](docs/resources/tenant.md), [`cozystack_redis`](docs/resources/redis.md)
- Data sources: [`cozystack_tenant`](docs/data-sources/tenant.md), [`cozystack_redis`](docs/data-sources/redis.md)

## Requirements

- Terraform >= 1.0 or OpenTofu >= 1.6
- A running Cozystack cluster and a kubeconfig (or bearer token) that can reach its API

## Using the provider

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
```

## Authentication

Connection settings mirror the official `kubernetes` provider, so existing kubeconfig and environment conventions transfer directly. Every provider attribute is optional and falls back to a `KUBE_*` environment variable: `config_path` (`KUBE_CONFIG_PATH`, then `KUBECONFIG`), `config_context` (`KUBE_CTX`), `host` (`KUBE_HOST`), `token` (`KUBE_TOKEN`), `cluster_ca_certificate` (`KUBE_CLUSTER_CA_CERT_DATA`), and `insecure` (`KUBE_INSECURE`). Set `in_cluster = true` to use a pod's service account instead of a kubeconfig.

## Development

```sh
make build   # build the provider binary
make test    # unit tests with the race detector
make lint    # golangci-lint
make docs    # regenerate docs/ with tfplugindocs
```

Acceptance tests create and destroy real tenants and need a reachable Cozystack cluster:

```sh
KUBECONFIG=/path/to/kubeconfig KUBE_CTX=my-context make testacc
```

The acceptance harness drives the Terraform CLI. To run it against OpenTofu instead, the `testacc` target points `TF_ACC_TERRAFORM_PATH` at `tofu` automatically.

## License

[BSD-3-Clause](LICENSE)
