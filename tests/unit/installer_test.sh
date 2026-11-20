#!/usr/bin/env bash
#
# installer.sh never links hm to bin/run, and moves a link it made earlier to bin/run onto bin/hm.
#
# bin/run, the launcher hm used to be, is gone, so a link to it is an installation that does
# nothing. Each case runs the installer against a HOME and a link folder of its own, with fake curl,
# uname, go, jq and sudo first on PATH: nothing reaches the network, a real toolchain or a system
# folder.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

INSTALLER="$HM_TEST_PROJECT_ROOT/installer.sh"
LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"; hm_test_home_cleanup' EXIT

# Tools every case has. go lives apart so a case can leave it out.
TOOLS="$LAB/tools"
GO_TOOLS="$LAB/go-tools"
mkdir -p "$TOOLS" "$GO_TOOLS"

# curl serves $FAKE_RELEASE/<asset name> and logs every URL; anything missing is a 404
cat > "$TOOLS/curl" <<'EOF'
#!/usr/bin/env bash
out="" url=""
while [ $# -gt 0 ]; do
    case "$1" in
        -o) out="$2"; shift ;;
        -*) ;;
        *)  url="$1" ;;
    esac
    shift
done
echo "$url" >> "$FAKE_CURL_LOG"
[ -f "$FAKE_RELEASE/$(basename "$url")" ] || exit 22
cp "$FAKE_RELEASE/$(basename "$url")" "$out"
EOF
printf '#!/usr/bin/env bash\ncase "$1" in -s) echo Linux ;; -m) echo x86_64 ;; *) echo Linux ;; esac\n' > "$TOOLS/uname"
printf '#!/usr/bin/env bash\necho jq-1.7\n' > "$TOOLS/jq"
printf '#!/usr/bin/env bash\necho "sudo was called: $*" >&2\nexit 1\n' > "$TOOLS/sudo"
cat > "$GO_TOOLS/go" <<'EOF'
#!/usr/bin/env bash
while [ $# -gt 0 ]; do
    [ "$1" == "-o" ] && { printf '#!/bin/sh\necho built\n' > "$2"; exit 0; }
    shift
done
exit 1
EOF
chmod +x "$TOOLS"/* "$GO_TOOLS"/*

BASE_PATH="/usr/bin:/bin"

# A real go reachable through the system folders would make "no toolchain" untestable
if PATH="$TOOLS:$BASE_PATH" command -v go >/dev/null 2>&1; then
    NO_GO_TESTABLE=false
else
    NO_GO_TESTABLE=true
fi

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

# new_case <name>: a HOME with a checkout at ~/hm whose HEAD is the tag 2.0.0, an empty release
# and a link folder, all of its own
new_case() {
    CASE="$LAB/$1"
    export HOME="$CASE/home"
    export HM_LINK="$CASE/prefix/bin/hm"
    export FAKE_RELEASE="$CASE/release"
    export FAKE_CURL_LOG="$CASE/curl.log"
    mkdir -p "$HOME/hm/bin" "$CASE/prefix/bin" "$FAKE_RELEASE"
    : > "$FAKE_CURL_LOG"

    printf '#!/usr/bin/env bash\necho shell-launcher\n' > "$HOME/hm/bin/run"
    printf '#!/usr/bin/env bash\ntouch "$HOME/completion-generated"\n' > "$HOME/hm/generate_completion.sh"
    chmod +x "$HOME/hm/bin/run" "$HOME/hm/generate_completion.sh"
    git -C "$HOME/hm" init -q
    git -C "$HOME/hm" add -A
    git -C "$HOME/hm" -c user.email=t@t -c user.name=t commit -qm first
    git -C "$HOME/hm" tag 2.0.0
}

publish() {
    printf 'release binary\n' > "$FAKE_RELEASE/hm_linux_amd64"
    echo "$(sha256_of "$FAKE_RELEASE/hm_linux_amd64")  hm_linux_amd64" > "$FAKE_RELEASE/checksums.txt"
}

install() {
    local path="$TOOLS:$BASE_PATH"
    [ "${1:-}" == "with-go" ] && path="$GO_TOOLS:$path"
    ( PATH="$path" bash "$INSTALLER" >"$CASE/out" 2>"$CASE/err" </dev/null )
    STATUS=$?
    STDERR=$(cat "$CASE/err")
}

link_of() {
    readlink "$HM_LINK" 2>/dev/null || echo "(no link)"
}

# ------------------------------------------------------------------ a release

new_case release
publish
install

test_case "a checkout at a tag gets that release's binary"
assert_equals "0" "$STATUS"
assert_equals "release binary" "$(cat "$HOME/hm/bin/hm")"

test_case "downloaded by tag, with its checksums"
assert_equals "https://github.com/hiberus-magento/hiberus-dockergento/releases/download/2.0.0/hm_linux_amd64
https://github.com/hiberus-magento/hiberus-dockergento/releases/download/2.0.0/checksums.txt" "$(cat "$FAKE_CURL_LOG")"

test_case "and hm is linked to it"
assert_equals "$HOME/hm/bin/hm" "$(link_of)"

test_case "nothing half-downloaded is left in bin/"
assert_equals "hm run" "$(ls "$HOME/hm/bin" | sort | tr '\n' ' ' | sed 's/ $//')"

test_case "the completion is still generated"
assert_equals "yes" "$([ -f "$HOME/completion-generated" ] && echo yes || echo no)"

# ------------------------------------------------------------------ an old link, built instead

new_case migrate-build
ln -s "$HOME/hm/bin/run" "$HM_LINK"
install with-go

test_case "a download that fails is built with the local toolchain"
assert_equals "0" "$STATUS"
assert_equals "built" "$("$HOME/hm/bin/hm")"

test_case "and a link to bin/run is moved to bin/hm"
assert_equals "$HOME/hm/bin/hm" "$(link_of)"

# ------------------------------------------------------------------ an old link, binary already there

new_case migrate-existing
printf '#!/bin/sh\necho present\n' > "$HOME/hm/bin/hm"
chmod +x "$HOME/hm/bin/hm"
ln -s "$HOME/hm/bin/run" "$HM_LINK"
install

test_case "a binary already there is linked without downloading"
assert_equals "0" "$STATUS"
assert_equals "$HOME/hm/bin/hm" "$(link_of)"
assert_equals "" "$(cat "$FAKE_CURL_LOG")"

# ------------------------------------------------------------------ a link somebody else made

new_case foreign
publish
ln -s /somewhere/else/hm "$HM_LINK"
install

test_case "a link the installer did not make is left alone"
assert_equals "/somewhere/else/hm" "$(link_of)"

# ------------------------------------------------------------------ no binary at all

if $NO_GO_TESTABLE; then
    new_case tampered
    publish
    printf 'tampered\n' > "$FAKE_RELEASE/hm_linux_amd64"
    ln -s "$HOME/hm/bin/run" "$HM_LINK"
    install

    test_case "a checksum mismatch with no toolchain fails"
    assert_equals "1" "$STATUS"
    assert_contains "$STDERR" "go build -o bin/hm ./cmd/hm"

    test_case "the tampered binary is not kept"
    assert_equals "no" "$([ -e "$HOME/hm/bin/hm" ] && echo yes || echo no)"

    test_case "and the existing link is left as it was"
    assert_equals "$HOME/hm/bin/run" "$(link_of)"

    # A checkout that is not at a release is only ever built, as hm update does: the latest
    # release's binary would be another version than the files beside it
    new_case branch
    publish
    git -C "$HOME/hm" -c user.email=t@t -c user.name=t commit -q --allow-empty -m "after the release"
    install

    test_case "a branch checkout with no toolchain is not given the latest release"
    assert_equals "1" "$STATUS"
    assert_equals "" "$(cat "$FAKE_CURL_LOG")"
    assert_equals "no" "$([ -e "$HOME/hm/bin/hm" ] && echo yes || echo no)"

    test_case "and says how to build it or pin a release"
    assert_contains "$STDERR" "go build -o bin/hm ./cmd/hm"
    assert_contains "$STDERR" "git -C $HOME/hm checkout"

    new_case nothing
    install

    test_case "a fresh install with no binary fails with how to get one"
    assert_equals "1" "$STATUS"
    assert_contains "$STDERR" "Install Go"

    test_case "and never links hm to bin/run"
    assert_equals "(no link)" "$(link_of)"
else
    echo "  - skipped the no-toolchain cases: a real go is in $BASE_PATH"
fi

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
