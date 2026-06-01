resource "cozystack_vpc" "net" {
  name      = "net"
  namespace = "tenant-root"

  subnets = [
    { name = "web", cidr = "10.0.0.0/24" },
    { name = "db", cidr = "10.0.1.0/24" },
  ]

  routes = [
    { cidr = "0.0.0.0/0", next_hop_ip = "10.0.0.1" },
  ]
}
