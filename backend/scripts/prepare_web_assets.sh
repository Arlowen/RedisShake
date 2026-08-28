#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BACKEND_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
REPOSITORY_DIR=$(CDPATH= cd -- "$BACKEND_DIR/.." && pwd)
FRONTEND_DIR="$REPOSITORY_DIR/frontend"

npm --prefix "$FRONTEND_DIR" run build
sh "$SCRIPT_DIR/sync_web_assets.sh"
echo "native Web assets prepared for Go embed.FS"
