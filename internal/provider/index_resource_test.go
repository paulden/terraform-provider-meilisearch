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
			// Create with a mix of flat and nested settings
			{
				Config: providerConfig + `
resource "meilisearch_index" "settings_test" {
	uid = "index-settings"
	primary_key = "id"

	ranking_rules         = ["words", "typo", "proximity"]
	searchable_attributes = ["title", "description"]
	filterable_attributes = ["category", "status"]
	sortable_attributes   = ["created_at"]
	stop_words            = ["the", "a", "an"]
	dictionary            = ["SQL"]
	search_cutoff_ms      = 150
	proximity_precision   = "byWord"
	separator_tokens      = ["|"]
	non_separator_tokens  = ["#"]

	synonyms = {
		"phone" = ["telephone", "mobile"]
	}

	typo_tolerance = {
		enabled = true
		min_word_size_for_typos = {
			one_typo  = 4
			two_typos = 8
		}
		disable_on_attributes = ["category"]
	}

	pagination = {
		max_total_hits = 500
	}

	faceting = {
		max_values_per_facet = 50
		sort_facet_values_by = {
			"*" = "alpha"
		}
	}

	localized_attributes = [
		{
			locales            = ["fra"]
			attribute_patterns = ["description"]
		}
	]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "uid", "index-settings"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "ranking_rules.#", "3"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "ranking_rules.0", "words"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "searchable_attributes.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "filterable_attributes.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "stop_words.#", "3"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "search_cutoff_ms", "150"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "proximity_precision", "byWord"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "typo_tolerance.enabled", "true"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "typo_tolerance.min_word_size_for_typos.one_typo", "4"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "pagination.max_total_hits", "500"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "faceting.max_values_per_facet", "50"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "localized_attributes.#", "1"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "localized_attributes.0.locales.#", "1"),
				),
			},
			// Update settings
			{
				Config: providerConfig + `
resource "meilisearch_index" "settings_test" {
	uid = "index-settings"
	primary_key = "id"

	ranking_rules         = ["words", "typo"]
	searchable_attributes = ["title"]
	filterable_attributes = ["category"]
	sortable_attributes   = ["created_at", "updated_at"]
	stop_words            = ["the"]
	distinct_attribute    = "category"

	typo_tolerance = {
		enabled = false
	}

	pagination = {
		max_total_hits = 1000
	}
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "ranking_rules.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "searchable_attributes.#", "1"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "sortable_attributes.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "distinct_attribute", "category"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "typo_tolerance.enabled", "false"),
					resource.TestCheckResourceAttr("meilisearch_index.settings_test", "pagination.max_total_hits", "1000"),
				),
			},
		},
	})
}

func TestAccIndexResourceWithEmbedders(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "meilisearch_index" "embedders_test" {
	uid = "index-embedders"
	primary_key = "id"

	embedders = {
		"default" = {
			source            = "userProvided"
			dimensions        = 512
			document_template = "{{doc.title}}"
		}
	}
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.embedders_test", "embedders.default.source", "userProvided"),
					resource.TestCheckResourceAttr("meilisearch_index.embedders_test", "embedders.default.dimensions", "512"),
				),
			},
		},
	})
}
