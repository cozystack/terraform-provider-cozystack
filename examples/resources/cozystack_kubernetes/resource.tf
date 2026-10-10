# storage_class is deliberately absent: the platform supplies its own, and a
# configured value makes any later change a cluster replacement, since a
# PersistentVolumeClaim cannot move to another class.
resource "cozystack_kubernetes" "cluster" {
  name      = "cluster"
  namespace = "tenant-root"

  version = "v1.35"

  # The admin kubeconfig is published asynchronously, so without waiting it is
  # only readable from the apply after the one that created the cluster.
  wait_for_ready = true
}

# Worker nodes are a separate resource: see cozystack_kubernetes_nodes.

# Every block below is optional and follows the platform default while unset,
# so pin only what you actually need to control.
resource "cozystack_kubernetes" "tuned" {
  name      = "tuned"
  namespace = "tenant-root"

  version = "v1.35"

  # Worker OS images pulled from a self-hosted image factory. The Talos release
  # and schematic stay unset, so they keep following the platform.
  talos = {
    image_factory_url    = "https://factory.example.test"
    installer_repository = "registry.example.test/installer"
  }

  # Platform identity, plus the bindings created inside the tenant cluster.
  oidc = {
    mode = "System"

    users = [
      { email = "alice@example.test", role = "admin" },
      { email = "bob@example.test", role = "view" },
    ]
  }
}

# A tenant-supplied issuer, and the escape hatch onto the kube-apiserver. Do not
# add --oidc-* flags there while oidc.mode is not None: the chart injects
# --authentication-config and the apiserver refuses to start with both.
resource "cozystack_kubernetes" "byo_identity" {
  name      = "byo-identity"
  namespace = "tenant-root"

  oidc = {
    mode = "CustomConfig"

    custom_config = {
      secret_ref = {
        name = "tenant-authentication-config"
      }
    }
  }

  control_plane = {
    api_server = {
      extra_args = ["--requestheader-uid-headers=X-Remote-Uid"]

      extra_volumes = [
        jsonencode({
          name      = "audit-policy"
          configMap = { name = "audit-policy" }
        }),
      ]

      extra_volume_mounts = [
        jsonencode({
          name      = "audit-policy"
          mountPath = "/etc/kubernetes/audit"
          readOnly  = true
        }),
      ]
    }
  }

  # Mirrored images, for an air-gapped or rate-limited registry.
  images = {
    kubectl          = "registry.example.test/kubectl:v1.35.0"
    talos_csr_signer = "registry.example.test/talos-csr-signer:v0.3.0"
  }
}
