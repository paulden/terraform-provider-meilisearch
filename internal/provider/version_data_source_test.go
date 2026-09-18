package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVersionDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + `
data "meilisearch_version" "test" {
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify all attributes are set
					resource.TestMatchResourceAttr("data.meilisearch_version.test", "commit_sha", regexp.MustCompile(`^[a-f0-9]{40}`)),
					resource.TestMatchResourceAttr("data.meilisearch_version.test", "commit_date", regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}.*|unknown)$`)),
					resource.TestMatchResourceAttr("data.meilisearch_version.test", "pkg_version", regexp.MustCompile(`^\d+\.\d+\.\d+`)),
					// Verify ID placeholder attribute is set
					resource.TestCheckResourceAttr("data.meilisearch_version.test", "id", "placeholder"),
				),
			},
		},
	})
}
