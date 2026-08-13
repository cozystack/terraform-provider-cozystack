# TenantGateway declares a tenant's per-namespace Gateway API / Cilium
# Gateway. The cozystack-controller reconciles the actual Gateway and
# per-listener Certificate resources from this CR.
resource "cozystack_tenant_gateway" "example" {
  name      = "example"
  namespace = "tenant-root"
  spec = jsonencode({
    apex               = "apps.example.com"
    certMode           = "dns01"
    issuerName         = "letsencrypt-prod"
    attachedNamespaces = ["tenant-root"]
    gatewayClassName   = "cilium"
    dns01 = {
      provider = "cloudflare"
      cloudflare = {
        apiTokenSecretRef = { name = "cloudflare-dns01-token", key = "token" }
      }
    }
  })
}
