#!/usr/bin/env bash
#
# Branch environments: names, profiles, registration and the overlay that expresses them.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

export HM_WORKTREE_DIR="$WORK/registry"
COMMAND_BIN_NAME="hm"
source "$TASKS_DIR/worktree_env.sh"

# ---------------------------------------------------------------- names
#
# A branch name is not a host name. It ends up in a domain, a volume name and a file name, and
# each of the three refuses something the others allow.

test_case "a branch becomes a name that can be a subdomain"
assert_equals "feature-checkout" "$(hm_worktree_slug "feature/checkout")"

test_case "case is not part of a host name"
assert_equals "feature-x" "$(hm_worktree_slug "Feature_X")"

test_case "no leading or trailing dashes"
assert_equals "hotfix" "$(hm_worktree_slug "/hotfix/")"

test_case "no repeated dashes"
assert_equals "a-b" "$(hm_worktree_slug "a///b")"

test_case "a branch with nothing usable in it leaves nothing"
assert_equals "" "$(hm_worktree_slug "///")"

# ---------------------------------------------------------------- profiles

test_case "lite is php alone"
assert_equals "phpfpm" "$(hm_worktree_profile_keeps lite)"

test_case "agent is a Magento that answers"
assert_equals "phpfpm nginx db search redis" "$(hm_worktree_profile_keeps agent)"

test_case "full keeps whatever the project has"
assert_equals "" "$(hm_worktree_profile_keeps full)"

test_case "an unknown profile is not a profile"
hm_worktree_profile_keeps nonsense >/dev/null 2>&1 && r=accepted || r=refused
assert_equals "refused" "$r"

# ---------------------------------------------------------------- the registration
#
# It lives outside the checkout because config/docker/properties.json is committed: the
# worktree's project name written there would travel in somebody's commit.

test_case "a registration can be written and read back"
hm_worktree_save "shop" "feature-x" "/code/shop-worktrees/feature-x" "feature/x" \
    "agent" "feature-x.shop.test" "shop-feature-x"
hm_worktree_load "shop" "feature-x"
assert_equals "/code/shop-worktrees/feature-x" "$WORKTREE_PATH"
assert_equals "feature/x" "$WORKTREE_BRANCH"
assert_equals "agent" "$WORKTREE_PROFILE"
assert_equals "shop-feature-x" "$WORKTREE_PROJECT"

test_case "it is registered outside the checkout"
assert_equals "0" "$([ -f "$WORK/registry/shop/feature-x.json" ] && echo 0 || echo 1)"

test_case "the project's environments can be listed"
hm_worktree_save "shop" "hotfix" "/code/shop-worktrees/hotfix" "hotfix" "lite" "hotfix.shop.test" "shop-hotfix"
assert_equals "feature-x hotfix" "$(hm_worktree_names shop | tr '\n' ' ' | sed 's/ $//')"

test_case "an unregistered name is not loaded"
hm_worktree_load "shop" "never-existed" && r=loaded || r=absent
assert_equals "absent" "$r"

test_case "forgetting one leaves the others"
hm_worktree_forget "shop" "hotfix"
assert_equals "feature-x" "$(hm_worktree_names shop | tr '\n' ' ' | sed 's/ $//')"

# ---------------------------------------------------------------- the overlay

SERVICES="phpfpm nginx db search redis varnish hitch mailhog rabbitmq"
OVERLAY="$WORK/agent.yml"
hm_worktree_write_overlay "$OVERLAY" "agent" "feature-x.shop.test" "shop-feature-x" "$SERVICES" "hm-gateway"

test_case "the agent profile removes what it does not run"
assert_contains "$(cat "$OVERLAY")" "varnish: !reset null"
assert_contains "$(cat "$OVERLAY")" "mailhog: !reset null"
assert_contains "$(cat "$OVERLAY")" "rabbitmq: !reset null"
assert_contains "$(cat "$OVERLAY")" "hitch: !reset null"

test_case "and keeps what it does"
assert_not_contains "$(cat "$OVERLAY")" "phpfpm: !reset null"
assert_not_contains "$(cat "$OVERLAY")" "search: !reset null"

test_case "no service publishes a port of its own"
assert_equals "0" "$(grep -c '^    ports:$' "$OVERLAY" || true)"
assert_contains "$(cat "$OVERLAY")" "ports: !reset []"

test_case "without varnish the address is served by nginx"
assert_contains "$(cat "$OVERLAY")" 'traefik.http.routers.shop-feature-x.rule: "Host(`feature-x.shop.test`)"'
assert_contains "$(cat "$OVERLAY")" 'loadbalancer.server.port: "8080"'

#
# A repeated key in YAML is not a merge: the last one wins and the earlier block disappears.
# That is how four services quietly kept their published ports when the proxy overlay was
# first written, so it is asserted here rather than discovered again.
#
test_case "every service appears exactly once"
duplicates=$(grep -E '^  [a-z0-9_-]+:' "$OVERLAY" | sort | uniq -d)
assert_equals "" "$duplicates"

FULL="$WORK/full.yml"
hm_worktree_write_overlay "$FULL" "full" "feature-x.shop.test" "shop-feature-x" "$SERVICES" "hm-gateway"

test_case "the full profile removes nothing"
assert_not_contains "$(cat "$FULL")" "!reset null"

test_case "and its address is served by varnish"
assert_contains "$(cat "$FULL")" 'loadbalancer.server.port: "6081"'

LITE="$WORK/lite.yml"
hm_worktree_write_overlay "$LITE" "lite" "feature-x.shop.test" "shop-feature-x" "$SERVICES" "hm-gateway"

test_case "a profile with no web service is routed nowhere"
assert_not_contains "$(cat "$LITE")" "traefik.enable"
assert_contains "$(cat "$LITE")" "nginx: !reset null"

# ---------------------------------------------------------------- dependencies
#
# Composer's autoloader computes its base directory from `dirname($vendorDir)`, and PHP resolves
# `__DIR__` to the real path behind a symlink. With a link, the worktree's own modules are never
# registered and the main checkout's code runs — verified with a real PHP. So they are mounted,
# and only while they are the same dependencies.

MAIN="$WORK/principal"
BRANCH="$WORK/rama"
mkdir -p "$MAIN/vendor/composer" "$MAIN/node_modules" "$BRANCH"
printf '{"packages": []}\n' > "$MAIN/composer.lock"

test_case "the same lock means the same dependencies, so they can be read"
cp "$MAIN/composer.lock" "$BRANCH/composer.lock"
hm_worktree_shares_vendor "$MAIN" "$BRANCH" && r=comparte || r=no
assert_equals "comparte" "$r"

test_case "a branch that changes them does not share"
printf '{"packages": [{"name": "algo/nuevo"}]}\n' > "$BRANCH/composer.lock"
hm_worktree_shares_vendor "$MAIN" "$BRANCH" && r=comparte || r=no
assert_equals "no" "$r"

test_case "and neither does a branch with no lock at all"
rm -f "$BRANCH/composer.lock"
hm_worktree_shares_vendor "$MAIN" "$BRANCH" && r=comparte || r=no
assert_equals "no" "$r"

test_case "a main checkout with no vendor has nothing to share"
cp "$MAIN/composer.lock" "$BRANCH/composer.lock"
rm -rf "$MAIN/vendor"
hm_worktree_shares_vendor "$MAIN" "$BRANCH" && r=comparte || r=no
assert_equals "no" "$r"
mkdir -p "$MAIN/vendor/composer"

test_case "the mounts put them where PHP looks for them"
MOUNTS=$(hm_worktree_vendor_mounts "$MAIN" "/var/www/html")
assert_contains "$MOUNTS" "$MAIN/vendor:/var/www/html/vendor:ro"
assert_contains "$MOUNTS" "$MAIN/node_modules:/var/www/html/node_modules:ro"

#
# Read-only is not caution for its own sake: it is what stops a `composer require` in one branch
# from corrupting the dependencies five other environments are reading.
#
test_case "and always read-only"
assert_equals "0" "$(printf '%s' "$MOUNTS" | grep -c -v ':ro$' || true)"

test_case "what is not there is not mounted"
rm -rf "$MAIN/node_modules"
assert_not_contains "$(hm_worktree_vendor_mounts "$MAIN" "/var/www/html")" "node_modules"

test_case "the overlay carries them on phpfpm, where PHP runs"
MOUNTED="$WORK/mounted.yml"
hm_worktree_write_overlay "$MOUNTED" "agent" "feature-x.shop.test" "shop-feature-x" \
    "$SERVICES" "hm-gateway" "$(hm_worktree_vendor_mounts "$MAIN" "/var/www/html")"
assert_contains "$(sed -n '/^  phpfpm:/,/^  [a-z]/p' "$MOUNTED")" "/vendor:ro"

test_case "and on nobody else"
assert_equals "1" "$(grep -c 'vendor:ro' "$MOUNTED")"

#
# The list is built with a command substitution, which eats the trailing newline: without one of
# its own the next service key lands on the same line and the document stops being YAML. It
# looked fine to a grep and failed on the first `docker compose config`.
#
test_case "and the line ends where it should"
assert_equals "" "$(grep -E 'vendor:ro.+' "$MOUNTED" || true)"


test_case "a worktree with its own dependencies gets no mount at all"
OWN="$WORK/own.yml"
hm_worktree_write_overlay "$OWN" "agent" "feature-x.shop.test" "shop-feature-x" \
    "$SERVICES" "hm-gateway" ""
assert_not_contains "$(cat "$OWN")" "vendor"

test_case "nothing is ever linked"
assert_equals "" "$(grep -n 'ln -s' "$COMMANDS_DIR/worktree.sh" || true)"

# ---------------------------------------------------------------- the recorded decision

test_case "the registration says whether they are shared"
hm_worktree_save "shop" "con-vendor" "/code/wt" "feature/x" "agent" "x.test" "shop-wt" "shared"
hm_worktree_load "shop" "con-vendor"
assert_equals "shared" "$WORKTREE_VENDOR"

test_case "and defaults to its own when nobody said"
hm_worktree_save "shop" "sin-vendor" "/code/wt2" "feature/y" "agent" "y.test" "shop-wt2"
hm_worktree_load "shop" "sin-vendor"
assert_equals "own" "$WORKTREE_VENDOR"

test_case "an older registration, written before any of this, reads as its own"
printf '{"path": "/code/wt3", "branch": "b", "profile": "agent", "domain": "z.test", "project": "p"}\n' \
    > "$WORK/registry/shop/antigua.json"
hm_worktree_load "shop" "antigua"
assert_equals "own" "$WORKTREE_VENDOR"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
