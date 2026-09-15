# Create a Meilisearch Index
resource "meilisearch_index" "example" {
	uid = "index-name"
	primary_key = "key-name"
}

# Create a Meilisearch Index with settings configured
resource "meilisearch_index" "example_with_settings" {
	uid         = "products"
	primary_key = "id"

	ranking_rules         = ["words", "typo", "proximity", "attribute", "sort", "exactness"]
	searchable_attributes = ["title", "description"]
	filterable_attributes = ["category", "status"]
	sortable_attributes   = ["created_at"]
	stop_words            = ["the", "a", "an"]
	distinct_attribute    = "sku"
	search_cutoff_ms      = 150
	proximity_precision   = "byWord"

	synonyms = {
		"phone"  = ["telephone", "mobile"]
		"laptop" = ["computer", "notebook"]
	}

	typo_tolerance = {
		enabled = true
		min_word_size_for_typos = {
			one_typo  = 4
			two_typos = 8
		}
		disable_on_attributes = ["sku"]
	}

	pagination = {
		max_total_hits = 1000
	}

	faceting = {
		max_values_per_facet = 100
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

	embedders = {
		"default" = {
			source            = "userProvided"
			dimensions        = 512
			document_template = "{{doc.title}} - {{doc.description}}"
		}
	}
}
