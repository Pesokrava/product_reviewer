#!/bin/bash

# List all products
# Usage: ./scripts/products/list.sh [limit] [offset]

BASE_URL="http://localhost:8080/api/v1"

LIMIT=${1:-20}
OFFSET=${2:-0}

curl -s "$BASE_URL/products?limit=$LIMIT&offset=$OFFSET" | jq .
