#!/bin/bash
set -e

echo "=== Running Mesh Platform Tests ==="

echo -n "Checking for pytest... "
if ! command -v pytest &> /dev/null; then
    echo "FAIL: pytest not found. Install with 'pip install -e .[dev]'."
    exit 1
fi
echo "OK"

echo "--- Unit + Integration Tests (skip e2e) ---"
pytest src/mesh -m "not e2e" -v

if [ "${RUN_E2E:-0}" = "1" ]; then
    echo "--- E2E Tests (require live cluster) ---"
    pytest src/mesh -m e2e -v
else
    echo "Skipping E2E tests. Set RUN_E2E=1 to include them."
fi

echo "=== All Tests Passed ==="
