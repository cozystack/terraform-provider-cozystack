resource "cozystack_tcpbalancer" "lb" {
  name      = "lb"
  namespace = "tenant-root"

  replicas       = 2
  whitelist_http = true
  whitelist      = ["192.0.2.0/24", "198.51.100.0/24"]
}
