resource "cozystack_vmdisk" "os" {
  name      = "vm-os"
  namespace = "tenant-root"
  storage   = "20Gi"
  source    = { image = { name = "ubuntu" } }
}

resource "cozystack_vminstance" "vm" {
  name      = "vm"
  namespace = "tenant-root"

  instance_type    = "u1.medium"
  instance_profile = "ubuntu"
  run_strategy     = "Always"

  gpus = [
    { class_name = "example-passthrough", count = 1 },
  ]

  disks = [
    { name = cozystack_vmdisk.os.name },
  ]

  external       = true
  external_ports = [22]
  ssh_keys       = ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA user@example.com"]

  cloud_init = <<-EOT
    #cloud-config
    packages:
      - qemu-guest-agent
  EOT

  # Addresses appear only once the guest is up, so without waiting they are only
  # readable from the apply after the one that created the VM. A guest that exits
  # before its address is read keeps the apply waiting until wait_timeout.
  wait_for_ready = true
}
