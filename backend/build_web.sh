#!/bin/sh

set -eu

BACKEND_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$BACKEND_DIR"

sh scripts/prepare_web_assets.sh
sh build.sh

VERSION=${VERSION:-dev}
COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}
GO_LDFLAGS="-X main.Version=${VERSION} -X main.GitCommit=${COMMIT}"

go build -trimpath -ldflags "$GO_LDFLAGS" -o bin/redis-shake-server ./cmd/redis-shake-server
echo "embedded Web server build success"
