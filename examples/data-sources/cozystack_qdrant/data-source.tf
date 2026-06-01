data "cozystack_qdrant" "vectors" {
  name      = "vectors"
  namespace = "tenant-root"
}

output "vectors_ready" {
  value = data.cozystack_qdrant.vectors.ready
}
