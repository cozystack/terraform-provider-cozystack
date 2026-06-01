resource "cozystack_foundationdb" "fdb" {
  name      = "fdb"
  namespace = "tenant-root"

  resources_preset = "c1.medium"
  storage          = { size = "32Gi" }
  image_type       = "unified"
}
