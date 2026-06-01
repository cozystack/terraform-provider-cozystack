resource "cozystack_bucket" "assets" {
  name      = "assets"
  namespace = "tenant-root"

  users = {
    app    = { readonly = false }
    backup = { readonly = true }
  }
}
