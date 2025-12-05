package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/meilisearch/meilisearch-go"
)

func TestAccKeyResource(t *testing.T) {
	client := meilisearch.New(getTestHost(), meilisearch.WithAPIKey("T35T-M45T3R-K3Y"))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + `
resource "meilisearch_key" "test" {
	uid = "66666666-7777-8888-9999-000000000000"
	name = "terraform_test_api_key"
	description = "Terraform acceptance tests API key"
	actions = ["search"]
  indexes = ["test_index_1", "test_index_2"]
	expires_at = "2042-04-02T00:42:42Z"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify all attributes are set
					resource.TestCheckResourceAttr("meilisearch_key.test", "uid", "66666666-7777-8888-9999-000000000000"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "name", "terraform_test_api_key"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "description", "Terraform acceptance tests API key"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "expires_at", "2042-04-02T00:42:42Z"),
					// Verifiy number and values of actions
					resource.TestCheckResourceAttr("meilisearch_key.test", "actions.#", "1"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "actions.0", "search"),
					// Verifiy number and values of indexes
					resource.TestCheckResourceAttr("meilisearch_key.test", "indexes.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "indexes.0", "test_index_1"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "indexes.1", "test_index_2"),
					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("meilisearch_key.test", "key"),
					resource.TestCheckResourceAttrSet("meilisearch_key.test", "created_at"),
					resource.TestCheckResourceAttrSet("meilisearch_key.test", "updated_at"),
				),
			},
			// ImportState testing
			{
				ResourceName:            "meilisearch_key.test",
				ImportStateId:           "66666666-7777-8888-9999-000000000000",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"id"},
			},
			// Update and Read testing
			{
				Config: providerConfig + `
resource "meilisearch_key" "test" {
	uid = "66666666-7777-8888-9999-000000000000"
	name = "terraform_test_api_key"
	description = "Terraform acceptance tests API key updated"
	actions = ["search"]
  indexes = ["test_index_1", "test_index_2"]
	expires_at = "2042-04-02T00:42:42Z"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify all attributes are set
					resource.TestCheckResourceAttr("meilisearch_key.test", "uid", "66666666-7777-8888-9999-000000000000"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "name", "terraform_test_api_key"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "description", "Terraform acceptance tests API key updated"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "expires_at", "2042-04-02T00:42:42Z"),
					// Verifiy number and values of actions
					resource.TestCheckResourceAttr("meilisearch_key.test", "actions.#", "1"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "actions.0", "search"),
					// Verifiy number and values of indexes
					resource.TestCheckResourceAttr("meilisearch_key.test", "indexes.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "indexes.0", "test_index_1"),
					resource.TestCheckResourceAttr("meilisearch_key.test", "indexes.1", "test_index_2"),
					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("meilisearch_key.test", "key"),
					resource.TestCheckResourceAttrSet("meilisearch_key.test", "created_at"),
					resource.TestCheckResourceAttrSet("meilisearch_key.test", "updated_at"),
				),
			},
			// Re-creating the key deleted outside of Terraform testing
			{
				PreConfig: func() {
					_, err := client.DeleteKey("66666666-7777-8888-9999-000000000000")
					if err != nil {
						return
					}
				},
				Config: providerConfig + `
resource "meilisearch_key" "test" {
	uid = "66666666-7777-8888-9999-000000000000"
	name = "terraform_test_api_key"
	description = "Terraform acceptance tests API key updated"
	actions = ["search"]
  indexes = ["test_index_1", "test_index_2"]
	expires_at = "2042-04-02T00:42:42Z"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
		},
	})
}
