#!/usr/bin/env bash

# Generate more work units for the standard worker leaf
# Set LEAF_ID and ADMIN_KEY before running

if [ -z "$LEAF_ID" ] || [ -z "$ADMIN_KEY" ]; then
    echo "Error: LEAF_ID and ADMIN_KEY environment variables must be set." >&2
    exit 1
fi

curl -X POST "https://lettuce-compute.gridlabs.science/api/v1/leafs/$LEAF_ID/work-units/generate" \
    -H 'Content-Type: application/json' -H "Authorization: Bearer $ADMIN_KEY" \
    -d '{
  "parameter_space": {
    "WorkerOptions": [
      {
        "ServerUrl": "https://inference.gridlabs.science",
        "TaskTime": "1440"
      }
    ]
  }
}' | jq
