resource "cozystack_openbao" "vault" {
  name      = "vault"
  namespace = "tenant-root"

  size = "10Gi"
  ui   = true
}
