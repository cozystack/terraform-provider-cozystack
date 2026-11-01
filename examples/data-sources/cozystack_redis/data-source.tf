data "cozystack_redis" "cache" {
  name      = "cache"
  namespace = "tenant-root"
}

output "cache_ready" {
  value = data.cozystack_redis.cache.ready
}

output "cache_maxmemory_policy" {
  value = data.cozystack_redis.cache.maxmemory_policy
}
