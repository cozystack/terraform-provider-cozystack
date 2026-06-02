# MarketplacePanel is a cluster-scoped dashboard CRD that controls how an
# application kind appears in the Cozystack marketplace UI. The spec is
# free-form (the dashboard preserves unknown fields).
resource "cozystack_marketplace_panel" "custom_app" {
  name = "custom-app"
  spec = jsonencode({
    apiGroup    = "apps.cozystack.io"
    apiVersion  = "v1alpha1"
    description = "My custom application"
    disabled    = false
    hidden      = false
  })
}
