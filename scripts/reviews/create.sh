#!/bin/bash

# Create a new review for a product
# Usage: ./scripts/reviews/create.sh <product_id> [first_name] [last_name] [review_text] [rating]
# Outputs: REVIEW_ID to stdout, followed by JSON response

BASE_URL="http://localhost:8080/api/v1"

PRODUCT_ID=$1
FIRST_NAME=${2:-"John"}
LAST_NAME=${3:-"Doe"}
REVIEW_TEXT=${4:-"This is a great product!"}
RATING_VAL=${5:-5} # Use a different variable name to avoid confusion

# Ensure RATING_VAL is treated as an integer
RATING=$((RATING_VAL + 0))

if [ -z "$PRODUCT_ID" ]; then
    echo "Usage: $0 <product_id> [first_name] [last_name] [review_text] [rating]" >&2
    exit 1
fi

CREATE_REVIEW_PAYLOAD="{
    \"product_id\": \"$PRODUCT_ID\",
    \"first_name\": \"$FIRST_NAME\",
    \"last_name\": \"$LAST_NAME\",
    \"review_text\": \"$REVIEW_TEXT\",
    \"rating\": $RATING
}"

echo "DEBUG: Product ID: $PRODUCT_ID"
echo "DEBUG: Generated JSON payload:"
echo "$CREATE_REVIEW_PAYLOAD" | jq .

RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d "$CREATE_REVIEW_PAYLOAD" "$BASE_URL/reviews")

REVIEW_ID=$(echo "$RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$REVIEW_ID" ]; then
    echo "Error: Failed to create review." >&2
    echo "$RESPONSE" >&2
    exit 1
fi

echo "REVIEW_ID=$REVIEW_ID"
echo "$RESPONSE" | jq .
