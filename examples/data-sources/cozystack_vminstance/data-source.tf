data "cozystack_vminstance" "vm" {
  name      = "vm"
  namespace = "tenant-root"
}

output "vminstance_gpus" {
  value = data.cozystack_vminstance.vm.gpus
}
