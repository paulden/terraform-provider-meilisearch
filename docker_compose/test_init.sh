MEILISEARCH_PORT=${MEILISEARCH_PORT:-7700}

curl \
  -X POST "http://localhost:${MEILISEARCH_PORT}/keys" \
  -H 'Authorization: Bearer T35T-M45T3R-K3Y' \
  -H 'Content-Type: application/json' \
  --data-binary '{
    "name": "test_api_key",
    "uid": "11111111-2222-3333-4444-555555555555",
    "description": "Test API key",
    "actions": ["documents.add"],
    "indexes": ["products", "users"],
    "expiresAt": "2042-04-02T00:42:42Z"
  }'

curl \
  -X POST "http://localhost:${MEILISEARCH_PORT}/indexes" \
  -H 'Authorization: Bearer T35T-M45T3R-K3Y' \
  -H 'Content-Type: application/json' \
  --data-binary '{
    "uid": "test_index",
    "primaryKey": "test_id"
  }'

curl \
  -X POST "http://localhost:${MEILISEARCH_PORT}/indexes" \
  -H 'Authorization: Bearer T35T-M45T3R-K3Y' \
  -H 'Content-Type: application/json' \
  --data-binary '{
    "uid": "test_index_no_primary_key"
  }'
