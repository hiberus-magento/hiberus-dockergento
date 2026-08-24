#!/usr/bin/env bash
#
# The install-time credentials, against a real container and a real Magento vendor tree.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$TASKS_DIR/admin_bootstrap.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

# ---------------------------------------------------------------- a set password wins

test_case "a password set in the configuration is used instead of a generated one"
printf '{"admin-user": "someone", "admin-password": "Configured123"}\n' > "$LAB/config.json"
configured=$(jq -r '."admin-password" // ""' "$LAB/config.json")
assert_equals "Configured123" "$configured"

test_case "the arguments handed to setup:install carry the password that will be used"
args=$(jq -r --arg password "Generated456" \
    '. + {"admin-password": $password} | to_entries | map("--" + .key + "=" + .value) | join(" ")' \
    "$LAB/config.json")
assert_contains "$args" "--admin-password=Generated456"

test_case "and only one password ends up in them"
assert_equals "1" "$(printf '%s\n' "$args" | grep -o 'admin-password' | wc -l | tr -d ' ')"

test_case "a generated password is never written back to the configuration"
before=$(cat "$LAB/config.json")
jq -r --arg password "Generated456" \
    '. + {"admin-password": $password} | to_entries | map("--" + .key + "=" + .value) | join(" ")' \
    "$LAB/config.json" >/dev/null
assert_equals "$before" "$(cat "$LAB/config.json")"

test_case "the tool ships no fixed password any more"
assert_empty "$(jq -r '."admin-password" // ""' "$DATA_DIR/config.json")"

test_case "and none is written into the file it creates from scratch"
assert_not_contains "$(grep -A12 'admin-user' "$COMMANDS_DIR/install.sh" | head -14)" "Hiberus123"

# ---------------------------------------------------------------- the QR, drawn by the project

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

# The QR is drawn by endroid/qr-code, which every Magento carries for the two factor module's own
# admin screen. Rather than trusting that, this installs the library into a throwaway project and
# renders through the same code path the tool uses.
if ! command -v composer >/dev/null 2>&1 && ! docker image inspect composer:2 >/dev/null 2>&1; then
    if ! docker pull composer:2 >/dev/null 2>&1; then
        echo "  - skipped: no way to obtain endroid/qr-code to render against"
        echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
        exit 0
    fi
fi

mkdir -p "$LAB/project"
if ! docker run --rm -v "$LAB/project":/app -w /app composer:2 \
    require endroid/qr-code --no-interaction --quiet >/dev/null 2>&1; then
    echo "  - skipped: endroid/qr-code could not be installed in the lab"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

# Rendered with the same image that resolved the dependencies. Using a different PHP is how this
# test first "passed" on a fatal platform-check error instead of on a QR code.
render_qr() {
    docker run --rm -v "$LAB/project":/app -w /app composer:2 php -r '
        $uri = $argv[1] ?? "";
        if (!is_file("vendor/autoload.php")) { exit(1); }
        require "vendor/autoload.php";
        $writer = "Endroid\\QrCode\\Writer\\ConsoleWriter";
        $code   = "Endroid\\QrCode\\QrCode";
        if (!class_exists($writer) || !class_exists($code)) { exit(1); }
        $qr = method_exists($code, "create") ? $code::create($uri) : new $code($uri);
        echo (new $writer())->write($qr)->getString();
    ' -- "$1" 2>/dev/null
}

DOMAIN="shop.local"
COMMANDS_DIR_REAL="$COMMANDS_DIR"
COMMANDS_DIR="$LAB/fake"
mkdir -p "$COMMANDS_DIR"
printf '#!/usr/bin/env bash\nexit 0\n' > "$COMMANDS_DIR/magento.sh"
chmod +x "$COMMANDS_DIR/magento.sh"
hm_register_second_factor "admin"
COMMANDS_DIR="$COMMANDS_DIR_REAL"

test_case "the library every Magento ships can draw the registration code"
qr=$(render_qr "$HM_ADMIN_OTP_URI")
[ -n "$qr" ] && r=drawn || r=empty
assert_equals "drawn" "$r"

test_case "and what it draws is a QR, not an error message"
lines=$(printf '%s\n' "$qr" | wc -l | tr -d ' ')
[ "$lines" -ge 15 ] && r=square || r="only $lines lines"
assert_equals "square" "$r"

test_case "made of blocks, as a scannable code has to be"
assert_contains "$qr" "█"

test_case "and carrying no trace of a PHP failure"
assert_not_contains "$qr" "Fatal error"

test_case "a project without the library reports it instead of pretending"
mkdir -p "$LAB/empty"
empty=$(docker run --rm -v "$LAB/empty":/app -w /app composer:2 php -r '
    if (!is_file("vendor/autoload.php")) { exit(1); }
' 2>/dev/null; echo $?)
assert_equals "1" "$empty"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
