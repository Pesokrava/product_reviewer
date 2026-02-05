#!/bin/bash

# Create a new product
# Usage: ./scripts/products/create.sh
# Outputs: PRODUCT_ID and PRODUCT_VERSION to stdout, followed by JSON response

BASE_URL="http://localhost:8080/api/v1"

CREATE_PRODUCT_PAYLOAD='{
    "name": "Test Product",
    "description": "A product for testing API calls.",
    "price": 19.99
}'

RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d "$CREATE_PRODUCT_PAYLOAD" "$BASE_URL/products")

PRODUCT_ID=$(echo "$RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
PRODUCT_VERSION=$(echo "$RESPONSE" | grep -o '"version":[0-9]*' | cut -d':' -f2)

if [ -z "$PRODUCT_ID" ]; then
    echo "Error: Failed to create product." >&2
    echo "$RESPONSE" >&2
    exit 1
fi

echo "PRODUCT_ID=$PRODUCT_ID"
echo "PRODUCT_VERSION=$PRODUCT_VERSION"
echo "$RESPONSE" | jq .
