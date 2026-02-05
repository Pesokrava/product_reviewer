#!/bin/bash

# Update a product
# Usage: ./scripts/products/update.sh <product_id> <product_version> [new_name] [new_price] [new_description]
# Outputs: JSON response

BASE_URL="http://localhost:8080/api/v1"

PRODUCT_ID=$1
PRODUCT_VERSION=$2
NEW_NAME=${3:-"Updated Test Product"}
NEW_PRICE=${4:-29.99}
NEW_DESCRIPTION=${5:-"An updated description for the test product."}

if [ -z "$PRODUCT_ID" ] || [ -z "$PRODUCT_VERSION" ]; then
    echo "Usage: $0 <product_id> <product_version> [new_name] [new_price] [new_description]" >&2
    exit 1
fi

UPDATE_PRODUCT_PAYLOAD="{
    \"name\": \"$NEW_NAME\",
    \"description\": \"$NEW_DESCRIPTION\",
    \"price\": $NEW_PRICE,
    \"version\": $PRODUCT_VERSION
}"

curl -s -X PUT -H "Content-Type: application/json" -d "$UPDATE_PRODUCT_PAYLOAD" "$BASE_URL/products/$PRODUCT_ID" | jq .
