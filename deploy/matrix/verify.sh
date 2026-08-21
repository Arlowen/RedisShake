#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPOSITORY_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
COMPOSE_FILE="$REPOSITORY_ROOT/deploy/compose.matrix.yaml"
MATRIX_CERT_DIR=$(mktemp -d /tmp/redisshake-matrix-certs.XXXXXX)
export MATRIX_CERT_DIR

cleanup() {
	status=$?
	trap - EXIT INT TERM
	if [ "$status" -ne 0 ]; then
		docker compose -f "$COMPOSE_FILE" logs --no-color >&2 || true
	fi
    docker compose -f "$COMPOSE_FILE" down --volumes --remove-orphans >/dev/null 2>&1 || true
    case "$MATRIX_CERT_DIR" in
        /tmp/redisshake-matrix-certs.*) rm -rf -- "$MATRIX_CERT_DIR" ;;
    esac
	exit "$status"
}
trap cleanup EXIT INT TERM

openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
    -subj '/CN=RedisShake Matrix CA' \
    -keyout "$MATRIX_CERT_DIR/ca.key" \
    -out "$MATRIX_CERT_DIR/ca.crt" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes \
    -subj '/CN=tls-redis' \
    -addext 'subjectAltName=DNS:tls-redis' \
    -keyout "$MATRIX_CERT_DIR/server.key" \
    -out "$MATRIX_CERT_DIR/server.csr" >/dev/null 2>&1
printf 'subjectAltName=DNS:tls-redis\n' > "$MATRIX_CERT_DIR/server.ext"
openssl x509 -req -days 1 \
    -in "$MATRIX_CERT_DIR/server.csr" \
    -CA "$MATRIX_CERT_DIR/ca.crt" \
    -CAkey "$MATRIX_CERT_DIR/ca.key" \
    -CAcreateserial \
    -extfile "$MATRIX_CERT_DIR/server.ext" \
    -out "$MATRIX_CERT_DIR/server.crt" >/dev/null 2>&1
chmod 0600 "$MATRIX_CERT_DIR"/*
chmod 0755 "$MATRIX_CERT_DIR"
chmod 0644 "$MATRIX_CERT_DIR/ca.crt" "$MATRIX_CERT_DIR/server.crt" "$MATRIX_CERT_DIR/server.key"

docker compose -f "$COMPOSE_FILE" up -d --wait

API_URL="http://127.0.0.1:${REDISSHAKE_MATRIX_UI_PORT:-18086}/api/v1/connections/test"

assert_success() {
    name=$1
    payload=$2
    response=$(curl --fail-with-body --silent --show-error \
        -H 'Content-Type: application/json' \
        --data "$payload" \
        "$API_URL")
    printf '%s' "$response" | jq -e '.success == true and ([.checks[].state] | index("FAIL") | not)' >/dev/null
    printf '%s: PASS\n' "$name"
}

assert_success 'ACL standalone' "$(jq -cn '{purpose:"target",connection:{name:"ACL",topology:"standalone",address:"acl-redis:6379",username:"sync-user",password:"matrix-pass"}}')"
assert_success 'Sentinel discovery' "$(jq -cn '{purpose:"source",connection:{name:"Sentinel",topology:"sentinel",address:"",sentinel:{address:"sentinel:26379",master_name:"matrix-master",tls:{enabled:false}},tls:{enabled:false}}}')"
assert_success 'Cluster MOVED routing' "$(jq -cn '{purpose:"target",connection:{name:"Cluster",topology:"cluster",address:"cluster-1:6379"}}')"
assert_success 'TLS CA verification' "$(jq -cn --rawfile ca "$MATRIX_CERT_DIR/ca.crt" '{purpose:"source",connection:{name:"TLS",topology:"standalone",address:"tls-redis:6379",tls:{enabled:true,server_name:"tls-redis",insecure_skip_verify:false,ca_cert_pem:$ca}}}')"
