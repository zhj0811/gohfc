#!/bin/bash
#
# Copyright PeerFintech. All Rights Reserved.
#
# Environment variables that affect this script:
# GO_TESTFLAGS: Flags are added to the go test command.
# GO_LDFLAGS: Flags are added to the go test command (example: -s).
# TEST_CHANGED_ONLY: Boolean on whether to only run tests on changed packages.

set -e

GO_CMD="${GO_CMD:-go}"
SCRIPT_DIR="$(dirname "$0")"
CONFIG_DIR=$(pwd)

GOMOD_PATH=$(cd ${SCRIPT_DIR} && ${GO_CMD} env GOMOD)
PROJECT_MODULE=$(awk -F' ' '$1 == "module" {print $2}' ${GOMOD_PATH})
PROJECT_DIR=$(dirname ${GOMOD_PATH})

MODULE="${MODULE:-${PROJECT_MODULE}}"
MODULE_PATH="${PROJECT_DIR}/${MODULE#${PROJECT_MODULE}}" && MODULE_PATH=${MODULE_PATH%/}
PKG_ROOT="${PKG_ROOT:-./}"


# Find all packages that should be tested.
cd "${MODULE_PATH}"
declare -a PKG_SRC=(
    "${PKG_ROOT}"
)

echo "Running unit tests..."

${GO_CMD} test -race -coverprofile=coverage.txt -covermode=atomic ${PKG_SRC} -p 1 -timeout=40m

echo "Unit tests finished successfully"
