resource "cozystack_scheduling_class" "gpu" {
  name = "gpu"
  spec = jsonencode({
    nodeSelector = { "nvidia.com/gpu.present" = "true" }
  })
}
