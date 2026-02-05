#!/bin/bash

# Delete a review
# Usage: ./scripts/reviews/delete.sh <review_id>

BASE_URL="http://localhost:8080/api/v1"

REVIEW_ID=$1

if [ -z "$REVIEW_ID" ]; then
  {
    echo "Usage: $0 <review_id>" >&2
    exit 1
  }
fi

curl -s -X DELETE "$BASE_URL/reviews/$REVIEW_ID"
