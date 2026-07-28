package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/provider"
)

// version is set by goreleaser at release time.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with debugger support")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		// NOTE: namespace must match the GitHub owner the provider is published
		// under (see README "Registry namespace").
		Address: "registry.terraform.io/uptimeeye/uptimeeye",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
