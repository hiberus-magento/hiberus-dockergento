#!/usr/bin/env bash
#
# The global proxy: the overlay it generates, and the routing it actually does.
#
# Two halves on purpose. The first checks what the generator writes, which is cheap and catches
# the mistakes that matter (a service left publishing a port, a duplicated YAML key). The second
# brings up a real Traefik and two real projects, because "routes by domain" is not something to
# take on trust.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/version.sh"

HM="$COMMAND_BIN_DIR/bin/run"

# Colima only mounts the real home, so the proxy's own directory has to live there or the
# container sees it empty. The test HOME is a throwaway elsewhere; this one is deliberate.
REAL_HOME=$(eval echo "~$(id -un)")
export HM_PROXY_DIR="$REAL_HOME/.hm/proxy-selftest"
LAB="$REAL_HOME/.hm/proxy-selftest-lab"

cleanup() {
    for project in proxyone proxytwo; do
        ( cd "$LAB/$project" 2>/dev/null &&
          docker compose -p "$project" -f docker-compose.yml -f docker-compose.proxy.yml down -v ) >/dev/null 2>&1
    done
    [ -f "$HM_PROXY_DIR/docker-compose.yml" ] &&
        docker compose -p hm-proxy -f "$HM_PROXY_DIR/docker-compose.yml" down >/dev/null 2>&1
    docker network rm hm-gateway >/dev/null 2>&1
    rm -rf "$LAB" "$HM_PROXY_DIR"
}
trap cleanup EXIT
cleanup

# ---------------------------------------------------------------- what the generator writes

overlay_for() {
    local dir="$LAB/generated"
    mkdir -p "$dir"
    (
        export REQUIREMENTS='{"mariadb":"10.6"}'
        export MAIL_SERVICE="${2:-mailhog}"
        export DOMAIN="$1"
        export COMPOSE_PROJECT_NAME="generated"
        export HM_PROXY_NETWORK="hm-gateway"
        export DOCKER_COMPOSE_FILE="$dir/docker-compose.yml"
        source "$TASKS_DIR/proxy.sh"
        hm_proxy_write_overlay
    )
    cat "$dir/docker-compose.proxy.yml"
}

overlay=$(overlay_for "shop.local")

test_case "every service stops publishing ports"
missing=""
for service in phpfpm nginx db redis varnish rabbitmq mailhog search; do
    printf '%s' "$overlay" | grep -A1 "^  $service:" | grep -q 'ports: !reset' || missing="$missing $service"
done
assert_empty "$missing"

test_case "no service is declared twice, which would silently drop the first block"
duplicated=$(printf '%s' "$overlay" | grep -E '^  [a-z]+:' | sort | uniq -d)
assert_empty "$duplicated"

test_case "hitch is removed, because the proxy terminates TLS now"
assert_contains "$overlay" "hitch: !reset null"

test_case "the storefront is routed on the project's domain"
assert_contains "$overlay" 'Host(`shop.local`)'

test_case "the mail interface gets its own subdomain"
assert_contains "$overlay" 'Host(`mail.shop.local`)'

test_case "and so do the queue and the search engine"
assert_contains "$overlay" 'Host(`queue.shop.local`)'
assert_contains "$overlay" 'Host(`search.shop.local`)'

test_case "the chosen mail catcher is the one routed"
pit=$(overlay_for "shop.local" "mailpit")
assert_contains "$pit" "  mailpit:"

test_case "routed services join the shared network"
assert_contains "$overlay" "- hm-gateway"

# ---------------------------------------------------------------- the real thing

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available for the routing checks"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

# A test that cannot run is better than a test that fails: the proxy needs 80 and 443, and a
# project that does not use it holds those itself.
# The proxy itself holds those ports when it is up, and this test manages its own
holder=$(docker ps --format '{{.Names}}\t{{.Ports}}' 2>/dev/null |
    awk -F'\t' '$2 ~ /:80->|:443->/ { print $1 }' | grep -v '^hm-proxy$' | head -1)

if [ -n "$holder" ]; then
    echo "  - skipped: '$holder' is using port 80 or 443"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

build_project() {
    local name="$1"
    local dir="$LAB/$name"
    mkdir -p "$dir"

    cat > "$dir/docker-compose.yml" <<YAML
services:
  varnish:
    image: hashicorp/http-echo
    command: ["-text=I am $name", "-listen=:6081"]
    ports: ["80:6081"]
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
    ports: ["3306:3306"]
YAML

    cat > "$dir/docker-compose.proxy.yml" <<YAML
services:
  db:
    ports: !reset []
  varnish:
    ports: !reset []
    networks: [default, hm-gateway]
    labels:
      traefik.enable: "true"
      traefik.docker.network: "hm-gateway"
      traefik.http.routers.$name.rule: "Host(\`$name.local\`)"
      traefik.http.routers.$name.entrypoints: "websecure"
      traefik.http.routers.$name.tls: "true"
      traefik.http.services.$name.loadbalancer.server.port: "6081"
networks:
  hm-gateway:
    external: true
YAML

    mkdir -p "$dir/config/docker"
    printf '{"MAGENTO_DIR": ".", "DOMAIN": "%s.local", "COMPOSE_PROJECT_NAME": "%s", "USE_PROXY": "true"}\n' \
        "$name" "$name" > "$dir/config/docker/properties.json"
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.mac.yml"
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.linux.yml"
}

(
    export COMMAND_BIN_NAME="$COMMAND_BIN_NAME"
    source "$TASKS_DIR/proxy.sh"
    hm_proxy_ensure_network
    hm_proxy_write_compose
    hm_proxy_write_certificate "proxyone.local"
    hm_proxy_write_certificate "proxytwo.local"
    docker compose -p hm-proxy -f "$HM_PROXY_DIR/docker-compose.yml" up -d
) >/dev/null 2>&1

build_project proxyone
build_project proxytwo

for project in proxyone proxytwo; do
    ( cd "$LAB/$project" &&
      docker compose -p "$project" -f docker-compose.yml -f docker-compose.proxy.yml up -d ) >/dev/null 2>&1
done

# Waiting on curl's exit code is useless here: a 404 from Traefik is a successful request, so the
# loop ended immediately and everything after it ran before the routers existed.
waited=0
until curl -sk --resolve "proxyone.local:443:127.0.0.1" "https://proxyone.local/" --max-time 3 2>/dev/null |
        grep -q "I am proxyone" || [ "$waited" -gt 90 ]; do
    sleep 3
    waited=$((waited + 3))
done

fetch() {
    curl -sk --resolve "$1:443:127.0.0.1" "https://$1/" --max-time 8 2>/dev/null | head -1
}

test_case "two projects are up at the same time"
running=$(docker ps --format '{{.Names}}' | grep -cE '^(proxyone|proxytwo)-varnish' || true)
assert_equals "2" "$running"

test_case "and each answers on its own domain, through the same port"
assert_contains "$(fetch proxyone.local)" "I am proxyone"

test_case "the other one too, which is the entire point"
assert_contains "$(fetch proxytwo.local)" "I am proxytwo"

test_case "neither publishes anything on the machine"
published=$(docker ps --format '{{.Ports}}' --filter "name=proxyone" --filter "name=proxytwo" |
    grep -c '\->' || true)
assert_equals "0" "$published"

test_case "the certificate served covers the wildcard, so subdomains work"
names=$(echo | openssl s_client -connect 127.0.0.1:443 -servername proxyone.local 2>/dev/null |
    openssl x509 -noout -ext subjectAltName 2>/dev/null)
assert_contains "$names" "*.proxyone.local"

test_case "the proxy reports what it routes"
status=$( "$HM" proxy status --json 2>/dev/null | jq -r '.data.routes[].host' )
assert_contains "$status" "proxyone.local"

# ---------------------------------------------------------------- the tunnel

test_case "the database is not reachable from the machine"
reachable=$(python3 -c "
import socket
s = socket.socket(); s.settimeout(2)
try:
    s.connect(('127.0.0.1', 3306)); print('yes')
except Exception:
    print('no')" 2>/dev/null)
assert_equals "no" "$reachable"

test_case "a tunnel makes it reachable"
output="$LAB/tunnel.out"
( cd "$LAB/proxyone" && nohup "$HM" tunnel db --no-json > "$output" 2>&1 & )
waited=0
until grep -q "127.0.0.1:" "$output" 2>/dev/null || [ "$waited" -gt 40 ]; do
    sleep 2
    waited=$((waited + 2))
done
tunnel_port=$(grep -oE '127\.0\.0\.1:[0-9]+' "$output" 2>/dev/null | head -1 | cut -d: -f2)
greeting=$(python3 -c "
import socket, sys
port = int('${tunnel_port:-0}' or 0)
if not port:
    print('no port'); sys.exit()
s = socket.socket(); s.settimeout(5)
try:
    s.connect(('127.0.0.1', port))
    print(s.recv(64)[5:20].decode('utf-8', 'replace').strip(chr(0)))
except Exception as error:
    print('failed:', error)" 2>/dev/null)
# The handshake opens with the server's version string — `5.5.5-10.6.28-M` — not with a name
case "$greeting" in
    [0-9]*.[0-9]*) result="handshake" ;;
    *)             result="$greeting" ;;
esac
assert_equals "handshake" "$result"

test_case "closing it leaves nothing behind"
( cd "$LAB/proxyone" && "$HM" tunnel --close ) >/dev/null 2>&1
assert_equals "0" "$(docker ps -aq --filter 'name=hm-tunnel' | grep -c . || true)"

test_case "a service the project does not have is refused"
( cd "$LAB/proxyone" && "$HM" tunnel nonexistent --json >"$LAB/out" 2>"$LAB/err" )
assert_json_field "$(cat "$LAB/err")" '.error.type' "unknown_service"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
