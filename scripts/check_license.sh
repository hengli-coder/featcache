#!/usr/bin/env bash
# check_license.sh — verify all Go source files carry a license header.
# Fails if any .go file is missing the Apache-2.0 header.
#
# Usage: bash scripts/check_license.sh [path...] (defaults to ./cmd ./pkg ./examples)

set -euo pipefail

HEADER='Licensed under the Apache License, Version 2.0'

DEFAULT_PATHS=(./cmd ./pkg ./examples)
if [ "$#" -gt 0 ]; then
    PATHS=("$@")
else
    PATHS=("${DEFAULT_PATHS[@]}")
fi

FILES=$(find "${PATHS[@]}" -name '*.go' -type f 2>/dev/null | grep -v _test.go || true)

if [ -z "$FILES" ]; then
    echo "No source files found to check."
    exit 0
fi

MISSING=0
for f in $FILES; do
    if ! grep -qF "$HEADER" "$f" 2>/dev/null; then
        echo "MISSING LICENSE HEADER: $f"
        MISSING=$((MISSING + 1))
    fi
done

if [ "$MISSING" -gt 0 ]; then
    echo "ERROR: $MISSING file(s) missing the Apache-2.0 license header."
    exit 1
fi

echo "OK: all source files carry the license header."
