resource "cozystack_application_definition" "example" {
  name = "example"
  spec = jsonencode({
    application = { kind = "Example", plural = "examples", singular = "example", openAPISchema = "" }
    release     = { prefix = "example-", chartRef = { kind = "HelmRepository", name = "example" } }
  })
}
