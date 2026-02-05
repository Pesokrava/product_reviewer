#!/bin/bash

# Get reviews for a product by product ID
# Usage: ./scripts/reviews/get_by_product.sh <product_id> [limit] [offset]

BASE_URL="http://localhost:8080/api/v1"

PRODUCT_ID=$1
LIMIT=${2:-20}
OFFSET=${3:-0}

if [ -z "$PRODUCT_ID" ]; then
    echo "Usage: $0 <product_id> [limit] [offset]" >&2
    exit 1
fi

curl -s "$BASE_URL/products/$PRODUCT_ID/reviews?limit=$LIMIT&offset=$OFFSET" | jq .
