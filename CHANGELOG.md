# Changelog

## v1.6.1

Tracks the Cozystack API at v1.6.1 (`apps.cozystack.io`), jumping straight from the 1.4 line; the 1.5 line is skipped.

### Breaking changes

- The `cozystack_marketplace_panel` resource and data source are removed. Cozystack 1.6 deleted the `dashboard.cozystack.io` API group, and the platform migration drops its CRDs during upgrade. Run `terraform state rm` on existing `cozystack_marketplace_panel` resources and remove them from configuration before upgrading.
- Changing a configured `storage_class` now replaces the object on every data-storing kind. The aggregated apiserver accepts an in-place change without migrating any volume, so the previous behaviour silently recorded a class the data does not live on.
- `cozystack_kubernetes` no longer defaults `storage_class` to `replicated` and no longer accepts `version = "v1.30"`, which the 1.6 server rejects.
- `rabbitmq`: the default `resources_preset` moves from `t1.nano` to `u1.nano`. The attribute is optional-with-default, so the old value sits in existing state: after upgrading the provider, a `cozystack_rabbitmq` that never set it explicitly plans `t1.nano` → `u1.nano`, which updates the cluster and rolls its pods. Set `resources_preset = "t1.nano"` explicitly to keep the previous sizing.

### New resources

- `cozystack_kubernetes_nodes` resource and data source for the new `KubernetesNodes` kind: standalone worker node pools managed independently of the parent cluster. The object must be named `<cluster>-<pool>`.
- `cozystack_tenant_gateway` resource and data source for the new `TenantGateway` kind (`gateway.cozystack.io`), as a raw-spec passthrough.

### Features

- `kubernetes`: new `talos`, `oidc`, `node_health_check`, `control_plane.api_server`, and `images` blocks; per-node-group `max_unhealthy` and `node_startup_timeout`; `resources.cpu` and `resources.memory` are validated both-or-neither at plan time; `node_groups = {}` and `roles = []` no longer fail apply.
- `tls` block with a tri-state `enabled` flag on `kafka`, `nats`, `qdrant`, and `postgres`. Unset inherits the `external` flag.
- `backup.use_system_bucket` opt-in on `postgres` and `clickhouse` for the platform-managed backup bucket; the upstream-deprecated per-tenant S3 fields stay unmanaged.
- `tenant`: `gateway` flag carrying the upstream three-state contract; an unset attribute leaves the spec key out.

### Design note

- No new 1.6 field carries a provider-side default, and resources store only configured values. The aggregated apiserver materialises chart defaults on every read; a provider that stored them would write them back as explicit spec keys and silently pin a cluster to the Talos release and schematic of its creation day.

## v1.4.3

The provider moved to the cozystack GitHub organization and now publishes under the `cozystack/cozystack` registry namespace. The tracked Cozystack API version (`apps.cozystack.io` v1.4.3) carries no schema changes relevant to the provider over v1.4.2.

### Breaking changes

- Registry source changed from `lexfrei/cozystack` to `cozystack/cozystack`. Update the `source` in your `required_providers` block (and any `dev_overrides`) accordingly.

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
