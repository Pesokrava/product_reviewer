#!/bin/bash

# Delete a product
# Usage: ./scripts/products/delete.sh <product_id>

BASE_URL="http://localhost:8080/api/v1"

PRODUCT_ID=$1

if [ -z "$PRODUCT_ID" ]; then
    echo "Usage: $0 <product_id>" >&2
    exit 1
fi

curl -s -X DELETE "$BASE_URL/products/$PRODUCT_ID"
