# Changelog

## v1.4.2

Initial release. The version tracks the Cozystack API version (`apps.cozystack.io` v1.4.2) the provider is built against.

### Managed resources and data sources

Tenant applications (`apps.cozystack.io`): `tenant`, `redis`, `qdrant`, `bucket`, `openbao`, `vpn`, `rabbitmq`, `mariadb`, `mongodb`, `clickhouse`, `nats`, `opensearch`, `postgres`, `httpcache`, `tcpbalancer`, `harbor`, `vpc`, `vmdisk`, `kafka`, `foundationdb`, `vminstance`, `kubernetes`.

Platform resources (`cozystack.io`): `package`, `package_source`, `application_definition`, `scheduling_class`.

Backups (`backups.cozystack.io`): `backup_class`, `backup`, `backup_job`, `backup_plan`, `restore_job`.

Tenant core (`core.cozystack.io`): `tenant_secret`, `tenant_module`, `tenant_namespace`. Dashboard: `marketplace_panel`.

### Features

- Server-generated outputs surfaced as computed (sensitive) attributes: `kubernetes.kubeconfig`, `postgres.endpoints`, `bucket.credentials`, `vminstance.ip_address`.
- `uid` (`metadata.uid`) exposed on every kind.
- Optional `wait_for_ready` / `wait_timeout` on managed resources.
- Authentication mirrors the kubernetes provider: kubeconfig (`config_path`/`config_context`), `host`/`token`, `cluster_ca_certificate`, `client_certificate`/`client_key`, `in_cluster`, and an `exec {}` credential-plugin block for OIDC login helpers.
