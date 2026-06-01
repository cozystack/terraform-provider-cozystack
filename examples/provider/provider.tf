terraform {
  required_providers {
    cozystack = {
      source = "lexfrei/cozystack"
    }
  }
}

# Authentication mirrors the official kubernetes provider. By default the
# provider reads the standard kubeconfig (KUBECONFIG / ~/.kube/config); the
# settings below are all optional and can also be supplied via KUBE_* env vars.
provider "cozystack" {
  config_path    = "~/.kube/config"
  config_context = "my-cozystack-cluster"
}
