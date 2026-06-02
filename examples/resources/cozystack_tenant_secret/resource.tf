resource "cozystack_tenant_secret" "app" {
  name      = "app-credentials"
  namespace = "tenant-root"
  type      = "Opaque"
  data = {
    username = "app"
    password = "change-me"
  }
}
