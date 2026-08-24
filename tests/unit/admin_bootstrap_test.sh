#!/usr/bin/env bash
#
# The credentials a fresh install leaves behind.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$TASKS_DIR/admin_bootstrap.sh"

# ---------------------------------------------------------------- the password

test_case "a password is generated"
hm_generate_admin_password
first="$HM_ADMIN_PASSWORD"
[ -n "$first" ] && r=generated || r=empty
assert_equals "generated" "$r"

test_case "two installs do not share a password"
hm_generate_admin_password
second="$HM_ADMIN_PASSWORD"
[ "$first" != "$second" ] && r=different || r="both were $first"
assert_equals "different" "$r"

test_case "it is long enough for Magento, and then some"
assert_equals "20" "${#first}"

test_case "letters and digits only, because it travels as an argument into the container"
case "$first" in
    *[!A-Za-z0-9]*) r="contains $first" ;;
    *)              r=alphanumeric ;;
esac
assert_equals "alphanumeric" "$r"

test_case "it always contains a digit, which Magento requires"
case "$first" in
    *[0-9]*) r=has_digit ;;
    *)       r="no digit in $first" ;;
esac
assert_equals "has_digit" "$r"

test_case "and always a letter"
case "$first" in
    *[A-Za-z]*) r=has_letter ;;
    *)          r="no letter in $first" ;;
esac
assert_equals "has_letter" "$r"

test_case "twenty of them in a row all differ"
seen=""
collisions=0
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    hm_generate_admin_password
    case "$seen" in
        *"$HM_ADMIN_PASSWORD"*) collisions=$((collisions + 1)) ;;
    esac
    seen="$seen $HM_ADMIN_PASSWORD"
done
assert_equals "0" "$collisions"

# ---------------------------------------------------------------- the OTP secret

test_case "a secret is generated"
hm_generate_otp_secret
secret="$HM_ADMIN_OTP_SECRET"
[ -n "$secret" ] && r=generated || r=empty
assert_equals "generated" "$r"

test_case "it is base32, which is what an authenticator app expects"
case "$secret" in
    *[!A-Z2-7]*) r="not base32: $secret" ;;
    *)           r=base32 ;;
esac
assert_equals "base32" "$r"

test_case "long enough to be a real secret"
assert_equals "32" "${#secret}"

test_case "and a different one each time"
hm_generate_otp_secret
[ "$secret" != "$HM_ADMIN_OTP_SECRET" ] && r=different || r=same
assert_equals "different" "$r"

# ---------------------------------------------------------------- the registration URI

test_case "the registration URI carries the secret and the issuer"
DOMAIN="shop.local"
COMMANDS_DIR="$HM_TEST_HOME/fake-commands"
mkdir -p "$COMMANDS_DIR"
cat > "$COMMANDS_DIR/magento.sh" <<'FAKE'
#!/usr/bin/env bash
exit 0
FAKE
chmod +x "$COMMANDS_DIR/magento.sh"
hm_register_second_factor "admin"
assert_contains "$HM_ADMIN_OTP_URI" "otpauth://totp/shop.local:admin"

test_case "and names the issuer separately, as the format requires"
assert_contains "$HM_ADMIN_OTP_URI" "issuer=shop.local"

test_case "the secret in the URI is the one that was registered"
assert_contains "$HM_ADMIN_OTP_URI" "secret=$HM_ADMIN_OTP_SECRET"

test_case "a failed registration reports it instead of showing a URI that does nothing"
cat > "$COMMANDS_DIR/magento.sh" <<'FAKE'
#!/usr/bin/env bash
exit 1
FAKE
chmod +x "$COMMANDS_DIR/magento.sh"
hm_register_second_factor "admin" && r=succeeded || r=refused
assert_equals "refused" "$r"

test_case "and leaves no URI behind"
assert_empty "$HM_ADMIN_OTP_URI"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
