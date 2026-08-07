# storage_class is deliberately absent: the platform supplies its own, and a
# configured value makes any later change a cluster replacement, since a
# PersistentVolumeClaim cannot move to another class.
resource "cozystack_kubernetes" "cluster" {
  name      = "cluster"
  namespace = "tenant-root"

  version = "v1.35"

  node_groups = {
    md0 = {
      instance_type = "u1.medium"
      disk_size     = "20Gi"
      min_replicas  = 1
      max_replicas  = 10
      roles         = ["ingress-nginx"]
    }
  }
}

# Every block below is optional and follows the platform default while unset,
# so pin only what you actually need to control.
resource "cozystack_kubernetes" "tuned" {
  name      = "tuned"
  namespace = "tenant-root"

  version = "v1.35"

  node_groups = {
    # Sized explicitly instead of by instance_type: cpu and memory go together.
    workers = {
      min_replicas = 2
      max_replicas = 6

      resources = {
        cpu    = "4"
        memory = "8Gi"
      }
    }

    # A GPU pool boots slower, so it overrides the cluster-wide remediation
    # timeout without changing it for every other group.
    gpu = {
      instance_type        = "u1.xlarge"
      min_replicas         = 0
      max_replicas         = 4
      node_startup_timeout = "30m"
      max_unhealthy        = "0%"
    }
  }

  # Worker OS images pulled from a self-hosted image factory. The Talos release
  # and schematic stay unset, so they keep following the platform.
  talos = {
    image_factory_url    = "https://factory.example.test"
    installer_repository = "registry.example.test/installer"
  }

  node_health_check = {
    max_unhealthy        = "50%"
    node_startup_timeout = "15m"
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

  node_groups = {}

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
