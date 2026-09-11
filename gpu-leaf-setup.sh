#!/usr/bin/env bash
# Configure + activate the pre-created GPU container leaf.
#
# The leaf already exists in DRAFT (id below). This fills in its execution/
# validation/data config (CONTAINER runtime, gpu_required=true) and activates it.
#
# Usage:
#   IMAGE_ID=your-registry/gpu-leaf:1.0 ./gpu-leaf-setup.sh
#   IMAGE_ID=your-registry/gpu-leaf@sha256:... ./gpu-leaf-setup.sh   # recommended: immutable digest
#
# For the local dry-run stack the defaults below work as-is. For a real head,
# export HEAD / ADMIN_KEY / LEAF_ID (or edit the defaults).
set -euo pipefail

: "${IMAGE_ID:?set IMAGE_ID to the container image reference, e.g. your-domain.com/gpu-leaf:1.0 or a @sha256 digest}"

HEAD="${HEAD:-https://lettuce-compute.gridlabs.science}"
ADMIN_KEY="${ADMIN_KEY:-dev-admin-key-not-for-production}"
LEAF_ID="${LEAF_ID:-ebb492bd-ca4c-4737-8769-f7dae060ac78}"

# 1) Move DRAFT -> CONFIGURING
curl -s -X POST "$HEAD/api/v1/leafs/$LEAF_ID/configure" \
  -H "Authorization: Bearer $ADMIN_KEY" >/dev/null

# 2) Set the execution + validation + data config (GPU container leaf)
#    - gpu_required: true  -> only GPU-capable volunteers pick up units
#    - comparison_mode NUMERIC_TOLERANCE is a safe default for GPU output
#      (GPU float math is not bit-identical across runs/devices); switch to
#      EXACT if your workload is deterministic. With redundancy_factor=1 no
#      comparison runs anyway.
curl -s -X PUT "$HEAD/api/v1/leafs/$LEAF_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{
    "execution_config": {
      "runtime": "CONTAINER",
      "image": "'"$IMAGE_ID"'",
      "gpu_required": true,
      "max_memory_mb": 8192,
      "max_disk_mb": 20480,
      "max_cpu_seconds": 3600,
      "network_access": true
    },
    "validation_config": {
      "redundancy_factor": 1,
      "agreement_threshold": 1.0,
      "comparison_mode": "NUMERIC_TOLERANCE",
      "numeric_tolerance": 0.01,
      "max_retries": 3
    },
    "fault_tolerance_config": {
      "deadline_multiplier": 3.0,
      "max_reassignments": 3
    },
    "data_config": {
      "transfer_strategy": "INLINE",
      "max_input_size_bytes": 1048576,
      "max_output_size_bytes": 10485760
    }
  }' >/dev/null

# 3) Activate (ACTIVE) so volunteers can pick it up
curl -s -X POST "$HEAD/api/v1/leafs/$LEAF_ID/activate" \
  -H "Authorization: Bearer $ADMIN_KEY" >/dev/null

echo "Leaf $LEAF_ID configured (gpu_required=true) with image '$IMAGE_ID' and activated."
echo "Next: generate work units, e.g."
echo "  curl -s -X POST \"$HEAD/api/v1/leafs/$LEAF_ID/work-units/generate\" \\"
echo "    -H 'Content-Type: application/json' -H \"Authorization: Bearer $ADMIN_KEY\" \\"
echo "    -d '{\"parameter_space\": {\"REPLACE\": [\"WITH\", \"YOUR\", \"PARAMS\"]}}'"
