#!/usr/bin/env bash
# check_coverage.sh — enforce a minimum coverage threshold on go test output.
#
# Usage:
#   bash scripts/check_coverage.sh <coverage-profile> [threshold-percent]
#
# Exits non-zero if total statement coverage is below the threshold.
# The profile is produced by: go test -coverprofile=coverage.out -covermode=atomic ./...

set -euo pipefail

PROFILE="${1:?usage: check_coverage.sh <coverage-profile> [threshold]}"
THRESHOLD="${2:-70}"

if [ ! -f "$PROFILE" ]; then
    echo "ERROR: coverage profile not found: $PROFILE"
    echo "Run: go test ./... -coverprofile=$PROFILE -covermode=atomic -count=1"
    exit 1
fi

# Extract total coverage: "total: (statements) 78.0%"
COVERAGE=$(go tool cover -func="$PROFILE" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')

if [ -z "$COVERAGE" ]; then
    echo "ERROR: could not parse coverage from $PROFILE"
    exit 1
fi

echo "Coverage: ${COVERAGE}% (threshold: ${THRESHOLD}%)"

# Compare as integers (round the percentage up to avoid false failures).
if awk -v c="$COVERAGE" -v t="$THRESHOLD" 'BEGIN { exit !(c + 0.5 < t) }'; then
    echo "ERROR: coverage ${COVERAGE}% is below threshold ${THRESHOLD}%"
    echo "Add tests before merging. New code must not reduce coverage."
    exit 1
fi

echo "OK: coverage meets threshold."
