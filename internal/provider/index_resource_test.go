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

	# Meilisearch rejects document_template for the userProvided source: it only
	# accepts source, dimensions, distribution and binaryQuantized.
	embedders = {
		"default" = {
			source     = "userProvided"
			dimensions = 512
		}
	}
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.embedders_test", "embedders.default.source", "userProvided"),
					resource.TestCheckResourceAttr("meilisearch_index.embedders_test", "embedders.default.dimensions", "512"),
				),
			},
			// Removing the embedders block resets it server-side.
			{
				Config: providerConfig + `
resource "meilisearch_index" "embedders_test" {
	uid = "index-embedders"
	primary_key = "id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("meilisearch_index.embedders_test", "embedders.%"),
					testCheckSettingAbsent("index-embedders", "embedders", "default"),
				),
			},
		},
	})
}

// TestAccIndexResourceSettingsUnset covers the two ways a practitioner stops
// managing a setting: emptying it, and removing it from the configuration
// entirely. Both must reach Meilisearch rather than lingering in state.
func TestAccIndexResourceSettingsUnset(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "meilisearch_index" "unset_test" {
	uid = "index-unset"
	primary_key = "id"

	stop_words            = ["the", "a"]
	searchable_attributes = ["title", "body"]
	sortable_attributes   = ["created_at"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.unset_test", "stop_words.#", "2"),
					resource.TestCheckResourceAttr("meilisearch_index.unset_test", "searchable_attributes.#", "2"),
					testCheckSettingJSON("index-unset", "stopWords", `["a","the"]`),
				),
			},
			// An explicit empty list must actually clear the setting server-side,
			// not be dropped by `omitempty` and silently keep the old value.
			{
				Config: providerConfig + `
resource "meilisearch_index" "unset_test" {
	uid = "index-unset"
	primary_key = "id"

	stop_words            = []
	searchable_attributes = ["title", "body"]
	sortable_attributes   = ["created_at"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.unset_test", "stop_words.#", "0"),
					testCheckSettingJSON("index-unset", "stopWords", `[]`),
				),
			},
			// Dropping attributes from the configuration resets them to the
			// Meilisearch defaults: ["*"] for searchableAttributes, [] for
			// sortableAttributes.
			{
				Config: providerConfig + `
resource "meilisearch_index" "unset_test" {
	uid = "index-unset"
	primary_key = "id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("meilisearch_index.unset_test", "stop_words.#"),
					resource.TestCheckNoResourceAttr("meilisearch_index.unset_test", "searchable_attributes.#"),
					testCheckSettingJSON("index-unset", "searchableAttributes", `["*"]`),
					testCheckSettingJSON("index-unset", "sortableAttributes", `[]`),
				),
			},
		},
	})
}

// TestAccIndexResourceUnmanagedSettings checks that settings this resource does
// not manage are left alone: changed out of band, they must not show up as
// drift and must not be reverted on the next apply.
func TestAccIndexResourceUnmanagedSettings(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "meilisearch_index" "unmanaged_test" {
	uid = "index-unmanaged"
	primary_key = "id"

	stop_words = ["the"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.unmanaged_test", "stop_words.#", "1"),
					resource.TestCheckNoResourceAttr("meilisearch_index.unmanaged_test", "ranking_rules.#"),
					// Set a setting the configuration does not mention.
					setSettingOutOfBand("index-unmanaged", "sortable-attributes", `["created_at"]`),
				),
			},
			{
				Config: providerConfig + `
resource "meilisearch_index" "unmanaged_test" {
	uid = "index-unmanaged"
	primary_key = "id"

	stop_words = ["the"]
}
`,
				// The out-of-band setting is not managed here, so it must not
				// appear as a diff...
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				// ...nor be reverted.
				Check: testCheckSettingJSON("index-unmanaged", "sortableAttributes", `["created_at"]`),
			},
		},
	})
}
