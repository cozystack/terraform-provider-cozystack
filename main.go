// Package main is the entrypoint for the Cozystack Terraform/OpenTofu provider plugin.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/cozystack/terraform-provider-cozystack/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// These are set at build time via -ldflags.
//
//nolint:gochecknoglobals // version and commit are injected at build time via -ldflags
var (
	version = "dev"
	commit  = "" //nolint:unused // populated by ldflags for release builds
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with debugger support")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/lexfrei/cozystack",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
