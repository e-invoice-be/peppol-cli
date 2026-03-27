#!/usr/bin/env bash
set -euo pipefail

# NOTE: oapi-codegen does not support OpenAPI 3.1 (the spec uses nullable union types).
# Types in internal/client/generated.go are maintained manually from api/openapi.json.
# When the spec changes, update generated.go to match.
#
# If oapi-codegen adds 3.1 support in the future, uncomment the command below:
#
# SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# ROOT_DIR="$(dirname "$SCRIPT_DIR")"
# "$(go env GOPATH)/bin/oapi-codegen" \
#   -generate types \
#   -package client \
#   -o "$ROOT_DIR/internal/client/generated.go" \
#   "$ROOT_DIR/api/openapi.json"

echo "NOTE: Types are manually maintained in internal/client/generated.go"
echo "      Update them when api/openapi.json changes."
