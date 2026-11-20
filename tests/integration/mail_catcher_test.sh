#!/usr/bin/env bash
#
# The mail catcher: the mail actually arriving, and what the binary reports about it.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
cleanup() {
    docker rm -f hm-mail-selftest >/dev/null 2>&1
    ( cd "$LAB/network" 2>/dev/null && docker compose -p hmmailalias down -v ) >/dev/null 2>&1
    rm -rf "$LAB"
}
trap cleanup EXIT

# What the rendered compose file says about the catcher —Mailhog by default, Mailpit when chosen,
# the same alias and port either way— is pinned in Go against core.RenderCompose and the real
# template (dockergento/core). What follows checks the mail actually arriving.

# ---------------------------------------------------------------- the mail actually arrives

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available for the delivery checks"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

# The image the Dockerfile builds on. Testing delivery against it is what makes the claim that
# Mailpit is a drop-in replacement something checked rather than assumed.
upstream=$(grep '^FROM' "$COMMAND_BIN_DIR/Dockerfiles/mailpit/1.0/Dockerfile" | awk '{print $2}')

if ! docker image inspect "$upstream" >/dev/null 2>&1; then
    if ! docker pull "$upstream" >/dev/null 2>&1; then
        echo "  - skipped: $upstream is not reachable"
        echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
        exit 0
    fi
fi

docker run --rm -d --name hm-mail-selftest -p 18025:8025 -p 11025:1025 \
    -e MP_SMTP_AUTH_ACCEPT_ANY=1 -e MP_SMTP_AUTH_ALLOW_INSECURE=1 "$upstream" >/dev/null 2>&1

waited=0
until curl -sf http://localhost:18025/api/v1/info >/dev/null 2>&1 || [ "$waited" -gt 30 ]; do
    sleep 1
    waited=$((waited + 1))
done

test_case "Mailpit answers on the port Mailhog used for its interface"
assert_equals "0" "$(curl -sf http://localhost:18025/api/v1/info >/dev/null 2>&1; echo $?)"

test_case "and accepts mail on the port Magento sends to"
python3 - <<'PY' >"$LAB/smtp" 2>&1
import smtplib
from email.message import EmailMessage
message = EmailMessage()
message["From"] = "magento@shop.local"
message["To"] = "customer@example.com"
message["Subject"] = "Order confirmation"
message.set_content("Your order has been received")
server = smtplib.SMTP("localhost", 11025)
# A local Magento sends with made-up credentials over a plain connection
server.login("magento", "not-a-real-password")
server.send_message(message)
server.quit()
print("sent")
PY
assert_contains "$(cat "$LAB/smtp")" "sent"

test_case "and the message is there to read"
assert_equals "1" "$(curl -s http://localhost:18025/api/v1/messages | jq -r '.total')"

docker rm -f hm-mail-selftest >/dev/null 2>&1

# ---------------------------------------------------------------- the alias, on a real network
#
# This is the claim the whole design rests on: a Magento installed against the host `mailhog`
# keeps delivering after switching to Mailpit. Checking it needs a network, so it runs a stack.

mkdir -p "$LAB/network"
cat > "$LAB/network/docker-compose.yml" <<YAML
services:
  mailpit:
    image: $upstream
    networks:
      default:
        aliases:
          - mailhog
  sender:
    image: alpine:latest
    command: ["sleep", "60"]
YAML

( cd "$LAB/network" && docker compose -p hmmailalias up -d ) >/dev/null 2>&1

test_case "a container looking for 'mailhog' finds Mailpit"
resolved=$( cd "$LAB/network" && docker compose -p hmmailalias exec -T sender \
    getent hosts mailhog 2>/dev/null | awk '{print $1}' | head -1 )
[ -n "$resolved" ] && r=resolved || r="mailhog does not resolve"
assert_equals "resolved" "$r"

test_case "and reaches its SMTP port under that name"
reachable=$( cd "$LAB/network" && docker compose -p hmmailalias exec -T sender \
    sh -c 'nc -z -w 3 mailhog 1025 && echo yes' 2>/dev/null )
assert_contains "$reachable" "yes"

( cd "$LAB/network" && docker compose -p hmmailalias down -v ) >/dev/null 2>&1

    # ---------------------------------------------------------------- what the CLI reports

HM="$COMMAND_BIN_DIR/bin/hm"
GO_BINARY="$COMMAND_BIN_DIR/bin/hm"

make_project() {
    local dir="$LAB/$1" service="$2" image="$3"
    mkdir -p "$dir/config/docker"
    cat > "$dir/docker-compose.yml" <<YAML
services:
  phpfpm:
    image: alpine:latest
  $service:
    image: $image
    ports:
      - 8025:8025
YAML
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.mac.yml"
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.linux.yml"
    printf '{"MAGENTO_DIR": ".", "DOMAIN": "mail.local", "MAIL_SERVICE": "%s"}\n' "$service" \
        > "$dir/config/docker/properties.json"
}

test_case "the mailbox address is reported under a key that does not name the implementation"
make_project reported mailpit "hiberusmagento/mailpit:1"
urls=$( cd "$LAB/reported" && "$GO_BINARY" describe --json 2>/dev/null | jq -r '.data.project.urls' )
assert_equals "http://localhost:8025" "$(printf '%s' "$urls" | jq -r '.mail')"

test_case "and the key that existed before still carries it"
assert_equals "http://localhost:8025" "$(printf '%s' "$urls" | jq -r '.mailhog')"

test_case "the diagnosis flags an image that cannot be obtained"
check=$( cd "$LAB/reported" && "$COMMAND_BIN_DIR/bin/hm" doctor --json 2>/dev/null |
    jq -r '.data.checks[] | select(.id | test("mail")) | .severity' )
assert_equals "error" "$check"

test_case "and says nothing when the image is there"
make_project fine mailhog "hiberusmagento/mailhog:1"
check=$( cd "$LAB/fine" && "$COMMAND_BIN_DIR/bin/hm" doctor --json 2>/dev/null |
    jq -r '.data.checks[] | select(.id | test("mail")) | .severity' )
assert_equals "ok" "$check"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
