data "cozystack_redis" "cache" {
  name      = "cache"
  namespace = "tenant-root"
}

output "cache_ready" {
  value = data.cozystack_redis.cache.ready
}
