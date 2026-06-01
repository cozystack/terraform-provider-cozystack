resource "cozystack_httpcache" "cdn" {
  name      = "cdn"
  namespace = "tenant-root"

  size      = "10Gi"
  endpoints = ["192.0.2.10:80", "192.0.2.11:80"]
}
