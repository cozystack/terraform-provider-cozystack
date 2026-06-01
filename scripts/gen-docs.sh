#!/usr/bin/env bash
# Regenerate docs/ from the provider schema using tfplugindocs.
#
# tfplugindocs normally downloads a Terraform CLI to export the schema. On
# machines without Terraform (or where a freshly downloaded binary is blocked,
# e.g. macOS Gatekeeper) we instead export the schema with whatever CLI is
# available — OpenTofu — via a dev override, and feed it to tfplugindocs with
# --providers-schema. The provider entry is re-keyed to the address tfplugindocs
# expects.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

TF_BIN="${TF_BIN:-$(command -v tofu || command -v terraform)}"
TFPLUGINDOCS="${TFPLUGINDOCS:-$(go env GOPATH)/bin/tfplugindocs}"

if [ -z "${TF_BIN}" ]; then
  echo "need tofu or terraform on PATH" >&2
  exit 1
fi

go build -o terraform-provider-cozystack .

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

cat > "${work}/dev.tfrc" <<EOF
provider_installation {
  dev_overrides { "registry.opentofu.org/lexfrei/cozystack" = "${ROOT}" }
  direct {}
}
EOF

cat > "${work}/main.tf" <<'EOF'
terraform {
  required_providers {
    cozystack = { source = "registry.opentofu.org/lexfrei/cozystack" }
  }
}
provider "cozystack" {}
EOF

(
  cd "${work}"
  TF_CLI_CONFIG_FILE="${work}/dev.tfrc" "${TF_BIN}" providers schema -json
) > "${work}/schema.json"

python3 - "${work}/schema.json" <<'PY'
import json, sys
path = sys.argv[1]
data = json.load(open(path))
schemas = data["provider_schemas"]
key = next(iter(schemas))
data["provider_schemas"] = {"registry.terraform.io/hashicorp/cozystack": schemas[key]}
json.dump(data, open(path, "w"))
PY

"${TFPLUGINDOCS}" generate --provider-name cozystack --providers-schema "${work}/schema.json"
echo "docs regenerated"
