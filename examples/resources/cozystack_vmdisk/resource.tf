resource "cozystack_vmdisk" "ubuntu" {
  name      = "ubuntu"
  namespace = "tenant-root"
  storage   = "20Gi"

  source = {
    http = { url = "https://example.com/images/ubuntu-22.04.img" }
  }
}
