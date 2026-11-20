#!/usr/bin/env bash
#
# Updating the tool and switching it between releases, by replacing the running executable.
#
# Offline: HM_RELEASES_URL points the binary at a fixture in the GitHub API's format, whose assets
# are file:// URLs to small scripts standing in for release binaries. Every case runs a fresh copy
# of the binary under test in a directory of its own, so nothing the developer has installed is
# ever replaced.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

if ! command -v go >/dev/null 2>&1; then
    echo "  - skipped: there is no Go toolchain to build the binary under test"
    echo "RESULT 0 0"
    exit 0
fi

LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'chmod -R u+w "$LAB" 2>/dev/null; rm -rf "$LAB"' EXIT

# The binary under test, stamped as release 2.0.0 the way GoReleaser stamps it, and once more
# unstamped, the way a checkout builds it
STAMP="github.com/hiberus-magento/hiberus-dockergento/internal/cli.Version"
( cd "$COMMAND_BIN_DIR" && go build -ldflags "-X $STAMP=2.0.0" -o "$LAB/hm-2.0.0" ./cmd/hm ) || exit 1
( cd "$COMMAND_BIN_DIR" && go build -o "$LAB/hm-devel" ./cmd/hm ) || exit 1

case "$(uname -s)" in Darwin) os=darwin ;; *) os=linux ;; esac
case "$(uname -m)" in arm64 | aarch64) arch=arm64 ;; *) arch=amd64 ;; esac
ASSET="hm_${os}_${arch}"

sha() { shasum -a 256 "$1" | cut -d' ' -f1; }

# release <tag>: a directory with the stand-in binary of <tag> for this platform and its checksums
release() {
    local dir="$LAB/releases/$1"
    mkdir -p "$dir"
    printf '#!/bin/sh\necho "fixture %s"\n' "$1" > "$dir/$ASSET"
    printf '%s  %s\n' "$(sha "$dir/$ASSET")" "$ASSET" > "$dir/checksums.txt"
}

# entry <tag> <prerelease>: the release as the GitHub API lists it
entry() {
    local dir="$LAB/releases/$1"
    printf '{"tag_name":"%s","prerelease":%s,"draft":false,"assets":[' "$1" "$2"
    printf '{"name":"%s","browser_download_url":"file://%s/%s"},' "$ASSET" "$dir" "$ASSET"
    printf '{"name":"checksums.txt","browser_download_url":"file://%s/checksums.txt"}]}' "$dir"
}

for tag in 2.2.0-rc.1 2.1.0 2.0.0 1.6.0; do release "$tag"; done

printf '[%s,%s,%s,%s]\n' "$(entry 2.2.0-rc.1 true)" "$(entry 2.1.0 false)" \
    "$(entry 2.0.0 false)" "$(entry 1.6.0 false)" > "$LAB/releases.json"
printf '[%s,%s]\n' "$(entry 2.0.0 false)" "$(entry 1.6.0 false)" > "$LAB/only-2.0.0.json"

# A release whose binary does not match its checksums
mkdir -p "$LAB/releases/tampered"
cp "$LAB/releases/2.1.0/checksums.txt" "$LAB/releases/tampered/checksums.txt"
printf '#!/bin/sh\necho "tampered"\n' > "$LAB/releases/tampered/$ASSET"
sed "s#$LAB/releases/2.1.0/$ASSET#$LAB/releases/tampered/$ASSET#" "$LAB/releases.json" > "$LAB/tampered.json"

export HM_RELEASES_URL="file://$LAB/releases.json"
unset GITHUB_TOKEN

# fresh [binary]: a copy of the binary under test, installed in a directory of its own
fresh() {
    rm -rf "$LAB/install"
    mkdir -p "$LAB/install"
    cp "$LAB/${1:-hm-2.0.0}" "$LAB/install/hm"
    HM="$LAB/install/hm"
    BEFORE=$(sha "$HM")
}

run() {
    ( cd "$LAB" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    STDERR=$(cat "$LAB/err")
    return 0
}

unchanged() { assert_equals "$BEFORE" "$(sha "$HM")"; }

# ------------------------------------------------------------------ listing

test_case "the releases of 2.0 and later are listed, newest first"
fresh
run switch --list --json
assert_json_field "$STDOUT" '[.data.versions[].name] | join(" ")' "2.2.0-rc.1 2.1.0 2.0.0"

test_case "the running version is the current one, and marked"
assert_json_field "$STDOUT" '.data.current' "2.0.0"
assert_json_field "$STDOUT" '[.data.versions[] | select(.installed) | .name] | join(" ")' "2.0.0"

test_case "a pre-release is marked as one, a final release is not"
assert_json_field "$STDOUT" '[.data.versions[] | select(.pre_release) | .name] | join(" ")' "2.2.0-rc.1"

test_case "the readable list labels the pre-releases and the installed version"
run switch --list --no-json
assert_contains "$STDOUT" "pre-release"
assert_contains "$STDOUT" "← installed"

# ------------------------------------------------------------------ update

test_case "update installs the newest final release over the running executable"
fresh
run update --no-json
assert_equals "0" "$STATUS"
assert_equals "fixture 2.1.0" "$("$HM")"
assert_contains "$STDOUT" "updated from 2.0.0 to 2.1.0"

test_case "update --pre takes the newest pre-release"
fresh
run update --pre --json
assert_json_field "$STDOUT" '.data.to' "2.2.0-rc.1"
assert_equals "fixture 2.2.0-rc.1" "$("$HM")"

test_case "an executable reached through a symlink is replaced where it really is"
fresh
mkdir -p "$LAB/install/bin"
ln -s "$LAB/install/hm" "$LAB/install/bin/hm"
( cd "$LAB" && "$LAB/install/bin/hm" update >/dev/null 2>&1 )
assert_equals "fixture 2.1.0" "$("$LAB/install/hm")"
assert_equals "true" "$([ -L "$LAB/install/bin/hm" ] && echo true || echo false)"

test_case "with nothing newer, update says it is up to date and changes nothing"
fresh
HM_RELEASES_URL="file://$LAB/only-2.0.0.json" run update --no-json
assert_equals "0" "$STATUS"
assert_contains "$STDOUT" "2.0.0 is up to date"
unchanged

test_case "a binary that does not match its checksum is refused, and the old one kept"
fresh
HM_RELEASES_URL="file://$LAB/tampered.json" run update --json
assert_equals "1" "$STATUS"
assert_json_field "$STDERR" '.error.type' "download_failed"
unchanged

test_case "releases that cannot be read are said"
fresh
HM_RELEASES_URL="file://$LAB/nowhere.json" run update --json
assert_equals "1" "$STATUS"
assert_json_field "$STDERR" '.error.type' "releases_unavailable"
unchanged

if [ "$(id -u)" -ne 0 ]; then
    test_case "a directory that cannot be written is said, with how to install there"
    fresh
    chmod 555 "$LAB/install"
    run update --json
    chmod 755 "$LAB/install"
    assert_equals "1" "$STATUS"
    assert_json_field "$STDERR" '.error.type' "install_dir_not_writable"
    assert_contains "$STDERR" "sudo"
    unchanged
fi

test_case "a binary built from a checkout is not updated"
fresh hm-devel
run update --json
assert_equals "6" "$STATUS"
assert_json_field "$STDERR" '.error.type' "built_from_source"
unchanged

# ------------------------------------------------------------------ switch

test_case "switch installs the release asked for"
fresh
run switch 2.1.0
assert_equals "0" "$STATUS"
assert_equals "fixture 2.1.0" "$("$HM")"

test_case "switch --stable installs the newest final release"
fresh
run switch --stable --json
assert_json_field "$STDOUT" '.data.version' "2.1.0"
assert_equals "fixture 2.1.0" "$("$HM")"

test_case "switching to the running version changes nothing"
fresh
run switch 2.0.0 --json
assert_equals "0" "$STATUS"
assert_json_field "$STDOUT" '.data.unchanged' "true"
unchanged

test_case "a version of the shell tool is refused, with how to install it separately"
fresh
run switch 1.6.0 --json
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "version_before_go"
assert_contains "$STDERR" "git clone -b 1.6.0"
unchanged

test_case "a branch is refused, with how to build it"
fresh
run switch release/2.0.0 --json
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "not_a_version"
assert_contains "$STDERR" "go build"
unchanged

test_case "an unknown version is refused"
fresh
run switch 9.9.9 --json
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "unknown_version"
unchanged

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
