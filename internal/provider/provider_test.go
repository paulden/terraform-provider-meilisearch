package provider

import (
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func getTestHost() string {
	port := os.Getenv("MEILISEARCH_PORT")
	if port == "" {
		port = "7700"
	}
	return fmt.Sprintf("http://localhost:%s", port)
}

var (
	providerConfig = fmt.Sprintf(`
provider "meilisearch" {
  host 		= "%s"
  api_key = "T35T-M45T3R-K3Y"
}
`, getTestHost())
)

var (
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"meilisearch": providerserver.NewProtocol6WithError(New("dev")()),
	}
)
