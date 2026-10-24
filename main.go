package main

import (
	"context"
	"flag"
	"log"

	"github.com/2chevskii/terraform-provider-hexcore/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// Set by -ldflags "-X main.version=..." when building a release.
var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Enable debugger support")
	flag.Parse()
	if err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/2chevskii/hexcore",
		Debug:   *debug,
	}); err != nil {
		log.Fatal(err)
	}
}
