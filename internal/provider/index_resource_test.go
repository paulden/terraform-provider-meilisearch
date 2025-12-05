package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccIndexResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + `
resource "meilisearch_index" "test" {
	uid = "index-uid"
	primary_key = "index-primary-key"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify all attributes are set
					resource.TestCheckResourceAttr("meilisearch_index.test", "uid", "index-uid"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "primary_key", "index-primary-key"),
					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("meilisearch_index.test", "created_at"),
					resource.TestCheckResourceAttrSet("meilisearch_index.test", "updated_at"),
				),
			},
			{
				Config: providerConfig + `
resource "meilisearch_index" "test" {
	uid = "index-uid"
	primary_key = "updated-index-primary-key"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("meilisearch_index.test", "Replace"),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.test", "primary_key", "updated-index-primary-key"),
				),
			},
			{
				Config: providerConfig + `
resource "meilisearch_index" "test" {
	uid = "updated-index-uid"
	primary_key = "updated-index-primary-key"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("meilisearch_index.test", "Replace"),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.test", "uid", "updated-index-uid"),
				),
			},
			{
				ResourceName:            "meilisearch_index.test",
				ImportStateId:           "updated-index-uid",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"id"},
			},
		},
	})
}

func TestAccIndexResourceWithSettings(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with settings
			{
				Config: providerConfig + `
resource "meilisearch_index" "test" {
	uid = "index-settings"
	primary_key = "id"

	ranking_rules = ["words", "typo", "proximity"]
	searchable_attributes = ["title", "description"]
	filterable_attributes = ["category", "status"]
	sortable_attributes = ["created_at"]
	stop_words = ["the", "a", "an"]
	synonyms = {
		"phone" = ["telephone", "mobile"]
		"laptop" = ["computer", "notebook"]
	}
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.test", "uid", "index-settings"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "primary_key", "id"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "ranking_rules.#", "3"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "ranking_rules.0", "words"),
					// For sets, we check the count but not the order
					resource.TestCheckResourceAttr("meilisearch_index.test", "searchable_attributes.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "filterable_attributes.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "stop_words.#", "3"),
				),
			},
			// Update settings
			{
				Config: providerConfig + `
resource "meilisearch_index" "test" {
	uid = "index-settings"
	primary_key = "id"

	ranking_rules = ["words", "typo"]
	searchable_attributes = ["title"]
	filterable_attributes = ["category"]
	sortable_attributes = ["created_at", "updated_at"]
	stop_words = ["the"]
	distinct_attribute = "category"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.test", "ranking_rules.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "searchable_attributes.#", "1"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "sortable_attributes.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.test", "distinct_attribute", "category"),
				),
			},
		},
	})
}
