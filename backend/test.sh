#!/bin/bash
set -e

# unit test
go test ./... -v

# black box test - use local pybbt module
cd tests/
export PYTHONPATH="$(pwd):$PYTHONPATH"
if [ -n "${PYTHON_BIN:-}" ]; then
    PYTHON_COMMAND=$PYTHON_BIN
elif command -v python >/dev/null 2>&1; then
    PYTHON_COMMAND=python
else
    PYTHON_COMMAND=python3
fi
TEST_FLAGS=${PYBBT_FLAGS-modules}
if [ -n "$TEST_FLAGS" ]; then
    "$PYTHON_COMMAND" -m pybbt cases --verbose --flags "$TEST_FLAGS"
else
    "$PYTHON_COMMAND" -m pybbt cases --verbose
fi
