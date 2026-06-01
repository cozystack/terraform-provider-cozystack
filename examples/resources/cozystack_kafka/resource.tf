resource "cozystack_kafka" "events" {
  name      = "events"
  namespace = "tenant-root"

  kafka     = { replicas = 3, size = "10Gi" }
  zookeeper = { replicas = 3, size = "5Gi" }

  topics = [
    { name = "orders", partitions = 6, replicas = 3 },
    { name = "payments", partitions = 3, replicas = 3 },
  ]
}
