#!/usr/bin/env bash
# Fail if total statement coverage in a Go cover profile is below a minimum.
# Usage: check-coverage.sh <coverage.out> <min-percent>
set -euo pipefail

profile="${1:?usage: check-coverage.sh <coverage.out> <min-percent>}"
min="${2:?usage: check-coverage.sh <coverage.out> <min-percent>}"

total=$(go tool cover -func="$profile" | awk '/^total:/ { sub("%", "", $3); print $3 }')
echo "Total coverage: ${total}% (minimum: ${min}%)"

if awk -v t="$total" -v m="$min" 'BEGIN { exit !(t < m) }'; then
  echo "::error::Coverage ${total}% is below the ${min}% minimum"
  exit 1
fi
