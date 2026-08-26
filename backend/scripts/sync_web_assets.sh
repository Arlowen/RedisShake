#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BACKEND_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
REPOSITORY_DIR=$(CDPATH= cd -- "$BACKEND_DIR/.." && pwd)
SOURCE_DIR="$REPOSITORY_DIR/frontend/dist"
TARGET_DIR="$BACKEND_DIR/internal/controlplane/webassets/dist"

test -f "$SOURCE_DIR/index.html"
find "$TARGET_DIR" -depth -mindepth 1 ! -name .placeholder -delete
cp -R "$SOURCE_DIR/." "$TARGET_DIR/"
test -f "$TARGET_DIR/index.html"
