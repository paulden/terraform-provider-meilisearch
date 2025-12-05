terraform {
  required_providers {
    meilisearch = {
      source = "hashicorp.com/edu/meilisearch"
    }
  }
}

provider "meilisearch" {
  host    = "http://localhost:7701"
  api_key = "masterKey"
}

resource "meilisearch_index" "example" {
  uid         = "test-settings-index"
  primary_key = "id"

  ranking_rules         = ["words", "typo", "proximity"]
  searchable_attributes = ["title", "description"]
  filterable_attributes = ["category", "status"]
  sortable_attributes   = ["created_at"]
  stop_words            = ["the", "a", "an"]

  synonyms = {
    "phone"  = ["telephone", "mobile"]
    "laptop" = ["computer", "notebook"]
  }
}
