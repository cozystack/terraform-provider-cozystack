resource "cozystack_bucket" "assets" {
  name      = "assets"
  namespace = "tenant-root"

  users = {
    app    = { readonly = false }
    backup = { readonly = true }
  }

  # S3 credentials are published asynchronously, so without waiting they are
  # only readable from the apply after the one that created the bucket.
  wait_for_ready = true
}
