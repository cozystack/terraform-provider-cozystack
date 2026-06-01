data "cozystack_bucket" "assets" {
  name      = "assets"
  namespace = "tenant-root"
}

output "assets_ready" {
  value = data.cozystack_bucket.assets.ready
}
