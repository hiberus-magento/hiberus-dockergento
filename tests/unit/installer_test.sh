#!/usr/bin/env bash
#
# install.sh installs hm 2 as one binary from a GitHub release: the latest of 2.0 or later, or the
# one asked for, verified against the release's checksums.txt. A directory that is not the user's
# gets only a link to the binary, which lives in ~/.hm/bin. A 1.x link is replaced at the same path,
# its completion line by the new one, and its checkout is left where it is with a notice.
# installer.sh, the old name, only runs install.sh.
#
# Each case runs against a HOME, a "system" bin directory (HM_SYSTEM_BIN, standing in for
# /usr/local/bin) and a release of its own, with fake curl, uname and sudo first on PATH: nothing
# reaches the network or a system folder.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

INSTALL_SH="$HM_TEST_PROJECT_ROOT/install.sh"
INSTALLER_SH="$HM_TEST_PROJECT_ROOT/installer.sh"
LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'chmod -R u+w "$LAB" 2>/dev/null; rm -rf "$LAB"; hm_test_home_cleanup' EXIT

TOOLS="$LAB/tools"
mkdir -p "$TOOLS"

# curl: the releases API from $FAKE_API (or the HTTP status $FAKE_API_STATUS), the releases/latest
# redirect to $FAKE_LATEST, assets from $FAKE_RELEASES/<tag>/<asset>, install.sh from the project;
# every URL logged with its headers (a -H @file read, and its mode logged), the raw arguments logged
# apart, anything missing a 404
#
# A download of a binary answers its headers (with -D) before its body, as a server does: its
# Content-Length, or none under FAKE_NO_LENGTH. FAKE_DOWNLOAD_DELAY waits before the body,
# FAKE_DOWNLOAD_HALVES writes half of it and waits before the rest, so that a watcher sees it grow.
cat >"$TOOLS/curl" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_CURL_ARGV"
out="" url="" headers="" write="" bar=false dump=""
while [ $# -gt 0 ]; do
    case "$1" in
        -o) out="$2"; shift ;;
        -D) dump="$2"; shift ;;
        --progress-bar) bar=true ;;
        -H)
            header="$2"
            if [[ "$header" == @* ]]; then
                ls -l "${header#@}" | cut -c1-10 >>"$FAKE_CURL_MODES"
                header="$(cat "${header#@}")"
            fi
            headers="$headers $header"; shift ;;
        -w) write="$2"; shift ;;
        --retry) shift ;;
        -*) ;;
        *) url="$1" ;;
    esac
    shift
done
echo "$url$headers" >>"$FAKE_CURL_LOG"
status() { [ "$write" == '%{http_code}' ] && printf '%s' "$1"; return 0; }
case "$url" in
    https://github.com/*/releases/latest)
        if [ -n "${FAKE_LATEST:-}" ]; then location="${url%/latest}/tag/$FAKE_LATEST"; else location="${url%/latest}"; fi
        [ "$write" == '%{redirect_url}' ] && printf '%s' "$location"
        exit 0 ;;
    https://api.github.com/*)
        [ -z "${FAKE_API_DELAY:-}" ] || sleep "$FAKE_API_DELAY"
        if [ -n "${FAKE_API_STATUS:-}" ]; then status "$FAKE_API_STATUS"; exit 22; fi
        source="$FAKE_API" ;;
    https://github.com/*/releases/download/*) rest="${url#*/releases/download/}"; source="$FAKE_RELEASES/$rest" ;;
    https://raw.githubusercontent.com/*/install.sh) source="$HM_TEST_PROJECT_ROOT/install.sh" ;;
    *) status 404; exit 22 ;;
esac
[ -f "$source" ] || { status 404; exit 22; }
size=$(wc -c <"$source" | tr -d ' ')
if [ -n "$dump" ]; then
    if [ -n "${FAKE_NO_LENGTH:-}" ]; then printf 'HTTP/1.1 200 OK\r\n\r\n' >"$dump"; else printf 'HTTP/1.1 302 Found\r\nContent-Length: 0\r\n\r\nHTTP/1.1 200 OK\r\nContent-Length: %s\r\n\r\n' "$size" >"$dump"; fi
fi
if [[ "$url" == */hm_* ]] && [ -n "$out" ]; then
    [ -z "${FAKE_DOWNLOAD_DELAY:-}" ] || sleep "$FAKE_DOWNLOAD_DELAY"
    if [ -n "${FAKE_DOWNLOAD_HALVES:-}" ]; then head -c $((size / 2)) "$source" >"$out"; sleep 0.6; fi
fi
if [ -n "$out" ]; then cp "$source" "$out"; else cat "$source"; fi
! $bar || printf '##################################################### 100.0%%\n' >&2
status 200
EOF
printf '#!/usr/bin/env bash\ncase "$1" in -s) echo "${FAKE_OS:-Linux}" ;; -m) echo x86_64 ;; *) echo "${FAKE_OS:-Linux}" ;; esac\n' >"$TOOLS/uname"
# sudo: logged, then run with the directory it was needed for made writable for the moment
cat >"$TOOLS/sudo" <<'EOF'
#!/usr/bin/env bash
echo "sudo $*" >>"$FAKE_SUDO_LOG"
chmod u+w "$HM_SYSTEM_BIN"
"$@"; status=$?
chmod u-w "$HM_SYSTEM_BIN"
exit $status
EOF
# jq: the installer must not need it (macOS ships one in /usr/bin, so it is shadowed, not omitted)
printf '#!/bin/sh\necho "jq $*" >>"$FAKE_JQ_LOG"\nexit 127\n' >"$TOOLS/jq"
chmod +x "$TOOLS"/*

BASE_PATH="/usr/bin:/bin"

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

# new_case <name>: HOME, system bin directory, release store and logs of its own
new_case() {
    CASE="$LAB/$1"
    export HOME="$CASE/home"
    export HM_SYSTEM_BIN="$CASE/usr-local-bin"
    export FAKE_RELEASES="$CASE/releases"
    export FAKE_API="$CASE/api.json"
    export FAKE_CURL_LOG="$CASE/curl.log"
    export FAKE_CURL_ARGV="$CASE/curl.argv"
    export FAKE_CURL_MODES="$CASE/curl.modes"
    export FAKE_SUDO_LOG="$CASE/sudo.log"
    export FAKE_JQ_LOG="$LAB/jq.log"
    export TMPDIR="$CASE/tmp"
    export SHELL=/bin/zsh
    unset HM_VERSION HM_PRE HM_INSTALL_DIR FAKE_OS GITHUB_TOKEN FAKE_LATEST FAKE_API_STATUS FAKE_NO_LENGTH FAKE_DOWNLOAD_DELAY FAKE_DOWNLOAD_HALVES
    mkdir -p "$HOME" "$HM_SYSTEM_BIN" "$FAKE_RELEASES" "$TMPDIR"
    : >"$FAKE_CURL_LOG"
    : >"$FAKE_CURL_ARGV"
    : >"$FAKE_CURL_MODES"
    : >"$FAKE_SUDO_LOG"
    printf '# my profile\nexport FOO=1\n' >"$HOME/.zshrc"
    api_releases
}

# publish <tag>: a release with a fake hm for linux/amd64 that reports that version
publish() {
    local dir="$FAKE_RELEASES/$1"
    mkdir -p "$dir"
    cat >"$dir/hm_linux_amd64" <<EOF
#!/bin/sh
case "\$1" in
    --version) printf 'hm $1\n  commit abc\n' ;;
    completion) echo "# hm \$2 completion of $1" ;;
    *) echo "hm $1 ran: \$*" ;;
esac
EOF
    echo "$(sha256_of "$dir/hm_linux_amd64")  hm_linux_amd64" >"$dir/checksums.txt"
}

# api_releases <tag[:draft]>...: what the releases API answers, newest first as GitHub orders it
api_releases() {
    local first=true tag draft
    {
        printf '[\n'
        for tag in "$@"; do
            draft=false
            [[ "$tag" == *:draft ]] && { draft=true; tag="${tag%:draft}"; }
            $first || printf ',\n'
            first=false
            printf '  {\n    "tag_name": "%s",\n    "name": "%s",\n    "draft": %s,\n    "prerelease": false,\n    "body": "notes with \\"tag_name\\": \\"9.9.9\\", inside"\n  }' "$tag" "$tag" "$draft"
        done
        printf '\n]\n'
    } >"$FAKE_API"
}

# run_install [args...]: install.sh as a file, with no terminal on stdin
run_install() {
    (PATH="$TOOLS:$BASE_PATH" bash "$INSTALL_SH" "$@" >"$CASE/out" 2>"$CASE/err" </dev/null)
    STATUS=$?
    OUT=$(cat "$CASE/out" "$CASE/err")
}

# run_install_refusing_sudo [args...]: run_install with a sudo that is there and always refuses
run_install_refusing_sudo() {
    mkdir -p "$CASE/no-sudo"
    printf '#!/bin/sh\necho "sudo $*" >>"$FAKE_SUDO_LOG"\nexit 1\n' >"$CASE/no-sudo/sudo"
    chmod +x "$CASE/no-sudo/sudo"
    (PATH="$CASE/no-sudo:$TOOLS:$BASE_PATH" bash "$INSTALL_SH" "$@" >"$CASE/out" 2>"$CASE/err" </dev/null)
    STATUS=$?
    OUT=$(cat "$CASE/out" "$CASE/err")
}

exists() { [ -e "$1" ] || [ -L "$1" ] && echo yes || echo no; }

# sudo_outside_system: each sudo command whose last argument, the file it writes, is not in the
# system directory; nothing when sudo only ever touched that directory
sudo_outside_system() { awk -v dir="$HM_SYSTEM_BIN/" 'index($NF, dir) != 1' "$FAKE_SUDO_LOG"; }

# ------------------------------------------------------------------ fresh install

new_case fresh
publish 2.0.0
publish 2.1.0-rc.1
publish 1.6.0
api_releases 2.1.0-rc.1 2.0.0 1.6.0 3.0.0:draft
run_install

test_case "a fresh install puts the latest final 2.x binary in the system bin directory"
assert_equals "0" "$STATUS"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_equals "no" "$([ -L "$HM_SYSTEM_BIN/hm" ] && echo yes || echo no)"

test_case "downloaded from that release with its checksums, drafts and 1.x ignored"
assert_contains "$(cat "$FAKE_CURL_LOG")" "releases/download/2.0.0/hm_linux_amd64"
assert_contains "$(cat "$FAKE_CURL_LOG")" "releases/download/2.0.0/checksums.txt"
assert_not_contains "$(cat "$FAKE_CURL_LOG")" "3.0.0"

test_case "no sudo when the directory is writable"
assert_equals "" "$(cat "$FAKE_SUDO_LOG")"

test_case "the completion is written by the binary and sourced from one profile line"
assert_equals "# hm zsh completion of 2.0.0" "$(cat "$HOME/.hm/completion/hm.zsh")"
assert_equals "1" "$(grep -c 'hm shell completion' "$HOME/.zshrc")"
assert_contains "$(cat "$HOME/.zshrc")" '. "$HOME/.hm/completion/hm.zsh"'
assert_contains "$(cat "$HOME/.zshrc")" "export FOO=1"

test_case "the summary says the version and the path"
assert_contains "$OUT" "hm 2.0.0 installed at $HM_SYSTEM_BIN/hm"

test_case "nothing is left behind in the temporary directory or next to the binary"
assert_equals "" "$(ls -A "$TMPDIR")"
assert_equals "hm" "$(ls -A "$HM_SYSTEM_BIN")"

test_case "without a terminal no progress is drawn: curl stays silent, no spinner, no lookup step"
assert_contains "$(grep hm_linux_amd64 "$FAKE_CURL_ARGV")" "-fsSL"
assert_not_contains "$(cat "$FAKE_CURL_ARGV")" "progress"
assert_not_contains "$(cat "$FAKE_CURL_ARGV")" "-#"
assert_not_contains "$OUT" "Looking up"
assert_not_contains "$OUT" "Latest release"

# ------------------------------------------------------------------ re-running

run_install

test_case "running it again is a no-op that says it is up to date"
assert_equals "0" "$STATUS"
assert_contains "$OUT" "hm 2.0.0 is already installed at $HM_SYSTEM_BIN/hm; it is up to date"
assert_not_contains "$(tail -n 2 "$FAKE_CURL_LOG")" "hm_linux_amd64"
assert_equals "1" "$(grep -c 'hm shell completion' "$HOME/.zshrc")"

publish 2.1.0
api_releases 2.1.0 2.0.0
run_install

test_case "and upgrades in place once there is a newer release"
assert_equals "0" "$STATUS"
assert_equals "hm 2.1.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_contains "$OUT" "upgraded hm 2.0.0 to 2.1.0"
assert_equals "1" "$(grep -c 'hm shell completion' "$HOME/.zshrc")"

# ------------------------------------------------------------------ piped from curl

new_case piped
publish 2.0.0
api_releases 2.0.0
(cd "$CASE" && cat "$INSTALL_SH" | PATH="$TOOLS:$BASE_PATH" bash >"$CASE/out" 2>"$CASE/err")
STATUS=$?

test_case "piped into bash it installs the same way"
assert_equals "0" "$STATUS"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"

new_case piped-args
publish 2.0.0
publish 2.1.0-rc.1
api_releases 2.1.0-rc.1 2.0.0
(cat "$INSTALL_SH" | PATH="$TOOLS:$BASE_PATH" bash -s -- --pre >"$CASE/out" 2>"$CASE/err")
STATUS=$?

test_case "and takes options after bash -s --; --pre allows the latest pre-release"
assert_equals "0" "$STATUS"
assert_equals "hm 2.1.0-rc.1" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"

new_case old-name
publish 2.0.0
api_releases 2.0.0
(cat "$INSTALLER_SH" | PATH="$TOOLS:$BASE_PATH" bash -s -- --version 2.0.0 >"$CASE/out" 2>"$CASE/err")
STATUS=$?

test_case "installer.sh, the old name piped from curl, fetches install.sh and runs it"
assert_equals "0" "$STATUS"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_contains "$(cat "$FAKE_CURL_LOG")" "raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh"

# ------------------------------------------------------------------ install directory

new_case not-writable
publish 2.0.0
api_releases 2.0.0
chmod u-w "$HM_SYSTEM_BIN"
run_install

test_case "a system directory that is not writable gets a link; the binary lives in ~/.hm/bin, owned by the user"
assert_equals "0" "$STATUS"
assert_equals "hm 2.0.0" "$("$HOME/.hm/bin/hm" --version | head -n 1)"
assert_equals "no" "$([ -L "$HOME/.hm/bin/hm" ] && echo yes || echo no)"
assert_equals "-rwxr-xr-x" "$(ls -l "$HOME/.hm/bin/hm" | cut -c1-10)"
assert_equals "$HOME/.hm/bin/hm" "$(readlink "$HM_SYSTEM_BIN/hm")"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_equals "no" "$(exists "$HOME/.local/bin/hm")"
assert_equals "hm" "$(ls -A "$HM_SYSTEM_BIN")"

test_case "sudo is used for the link only, never to write the binary"
assert_equals "yes" "$([ -s "$FAKE_SUDO_LOG" ] && echo yes || echo no)"
assert_equals "" "$(sudo_outside_system)"
assert_not_contains "$(cat "$FAKE_SUDO_LOG")" "cp "

test_case "the receipt records the binary and the link"
assert_equals "$HOME/.hm/bin/hm" "$(sed -n 's/^path=//p' "$HOME/.hm/install-receipt")"
assert_equals "$HM_SYSTEM_BIN/hm" "$(sed -n 's/^link=//p' "$HOME/.hm/install-receipt")"

test_case "the plain output names the layout"
assert_contains "$OUT" "hm 2.0.0 installed at $HOME/.hm/bin/hm"
assert_contains "$OUT" "the binary lives in $HOME/.hm/bin/hm; $HM_SYSTEM_BIN/hm links to it, so updates need no sudo"

: >"$FAKE_SUDO_LOG"
link_before=$(ls -di "$HM_SYSTEM_BIN/hm" | awk '{ print $1 }')
run_install

test_case "running it again with the link in place needs no sudo and leaves the link alone"
assert_equals "0" "$STATUS"
assert_contains "$OUT" "hm 2.0.0 is already installed"
assert_equals "" "$(cat "$FAKE_SUDO_LOG")"
assert_equals "$link_before" "$(ls -di "$HM_SYSTEM_BIN/hm" | awk '{ print $1 }')"

publish 2.1.0
api_releases 2.1.0 2.0.0
run_install

test_case "and an upgrade replaces ~/.hm/bin/hm only: no sudo, the link untouched"
assert_equals "0" "$STATUS"
assert_equals "hm 2.1.0" "$("$HOME/.hm/bin/hm" --version | head -n 1)"
assert_equals "hm 2.1.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_contains "$OUT" "upgraded hm 2.0.0 to 2.1.0"
assert_equals "" "$(cat "$FAKE_SUDO_LOG")"
assert_equals "$link_before" "$(ls -di "$HM_SYSTEM_BIN/hm" | awk '{ print $1 }')"
assert_equals "$HOME/.hm/bin/hm" "$(readlink "$HM_SYSTEM_BIN/hm")"
assert_equals "" "$(ls -A "$HOME/.hm/bin" | grep -v '^hm$')"

run_install --uninstall

test_case "--uninstall removes the link, through sudo, and ~/.hm/bin/hm"
assert_equals "0" "$STATUS"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"
assert_equals "no" "$(exists "$HOME/.hm/bin/hm")"
assert_contains "$(cat "$FAKE_SUDO_LOG")" "rm -f $HM_SYSTEM_BIN/hm"
assert_contains "$OUT" "hm was uninstalled"
chmod u+w "$HM_SYSTEM_BIN"

new_case not-writable-styled
publish 2.0.0
api_releases 2.0.0
chmod u-w "$HM_SYSTEM_BIN"
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
PLAIN=$(cat "$CASE/out" "$CASE/err" | sed $'s/\033\\[[0-9;]*m//g')
chmod u+w "$HM_SYSTEM_BIN"

test_case "the styled output names the layout too"
assert_equals "0" "$STATUS"
assert_contains "$PLAIN" "✓ Installed at $HOME/.hm/bin/hm"
assert_contains "$PLAIN" "✓ The binary lives in $HOME/.hm/bin/hm; $HM_SYSTEM_BIN/hm links to it, so updates need no sudo"

new_case not-writable-sudo-refused
publish 2.0.0
api_releases 2.0.0
chmod u-w "$HM_SYSTEM_BIN"
mkdir -p "$CASE/no-sudo"
printf '#!/bin/sh\necho "sudo $*" >>"$FAKE_SUDO_LOG"\nexit 1\n' >"$CASE/no-sudo/sudo"
chmod +x "$CASE/no-sudo/sudo"
(PATH="$CASE/no-sudo:$TOOLS:$BASE_PATH" bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
OUT=$(cat "$CASE/out" "$CASE/err")
chmod u+w "$HM_SYSTEM_BIN"

test_case "a fresh install whose sudo is refused falls back to ~/.local/bin, leaving no ~/.hm/bin/hm"
if [ "$(id -u)" -eq 0 ]; then
    echo "  - skipped: running as root, a read-only folder does not need sudo"
else
    assert_equals "0" "$STATUS"
    assert_equals "hm 2.0.0" "$("$HOME/.local/bin/hm" --version | head -n 1)"
    assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"
    assert_equals "no" "$(exists "$HOME/.hm/bin/hm")"
    assert_equals "" "$(ls -A "$HM_SYSTEM_BIN")"
    assert_contains "$(sed -n 's/^path=//p' "$HOME/.hm/install-receipt")" "$HOME/.local/bin/hm"
    assert_equals "" "$(sed -n 's/^link=//p' "$HOME/.hm/install-receipt")"
fi

test_case "with a warning when it is not on PATH"
assert_contains "$OUT" "$HOME/.local/bin is not in your PATH"

new_case rc-binary
publish 2.0.0
api_releases 2.0.0
run_install
chmod u-w "$HM_SYSTEM_BIN"
run_install

test_case "an earlier binary directly in a system directory that is not writable is moved to ~/.hm/bin, even up to date"
assert_equals "0" "$STATUS"
assert_equals "$HOME/.hm/bin/hm" "$(readlink "$HM_SYSTEM_BIN/hm")"
assert_equals "hm 2.0.0" "$("$HOME/.hm/bin/hm" --version | head -n 1)"
assert_equals "1" "$(grep -c 'hm_linux_amd64' "$FAKE_CURL_LOG")"
assert_contains "$OUT" "moved the binary from $HM_SYSTEM_BIN/hm to $HOME/.hm/bin/hm"
assert_equals "" "$(sudo_outside_system)"
assert_equals "hm" "$(ls -A "$HM_SYSTEM_BIN")"
chmod u+w "$HM_SYSTEM_BIN"

new_case rc-binary-upgrade
publish 2.0.0
api_releases 2.0.0
run_install
chmod u-w "$HM_SYSTEM_BIN"
publish 2.1.0
api_releases 2.1.0 2.0.0
run_install
chmod u+w "$HM_SYSTEM_BIN"

test_case "and converted on an upgrade: the new version in ~/.hm/bin/hm, a link in its place, sudo for the link only"
assert_equals "0" "$STATUS"
assert_equals "$HOME/.hm/bin/hm" "$(readlink "$HM_SYSTEM_BIN/hm")"
assert_equals "hm 2.1.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_contains "$OUT" "upgraded hm 2.0.0 to 2.1.0"
assert_equals "yes" "$([ -s "$FAKE_SUDO_LOG" ] && echo yes || echo no)"
assert_equals "" "$(sudo_outside_system)"
assert_equals "$HM_SYSTEM_BIN/hm" "$(sed -n 's/^link=//p' "$HOME/.hm/install-receipt")"

# An earlier binary the installer would move, and a sudo that refuses: up to date, or newer than
# the latest release, it is left where it is and still works, as before the link layout existed
for kept_case in up-to-date newer; do
    new_case "rc-binary-sudo-refused-$kept_case"
    publish 2.0.0
    publish 2.1.0
    api_releases 2.0.0
    if [ "$kept_case" == "newer" ]; then run_install --version 2.1.0; kept_version=2.1.0; else run_install; kept_version=2.0.0; fi
    rm -f "$HOME/.hm/completion/hm.zsh"
    chmod u-w "$HM_SYSTEM_BIN"
    run_install_refusing_sudo
    chmod u+w "$HM_SYSTEM_BIN"

    test_case "an earlier binary ($kept_case) whose move sudo refuses stays where it is: a warning, exit 0"
    if [ "$(id -u)" -eq 0 ]; then
        echo "  - skipped: running as root, a read-only folder does not need sudo"
        continue
    fi
    assert_equals "0" "$STATUS"
    assert_equals "no" "$([ -L "$HM_SYSTEM_BIN/hm" ] && echo yes || echo no)"
    assert_equals "hm $kept_version" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
    assert_equals "no" "$(exists "$HOME/.hm/bin/hm")"
    assert_equals "no" "$(exists "$HOME/.local/bin/hm")"
    assert_contains "$OUT" "updates will still need sudo until this installer is run again with sudo"
    assert_not_contains "$OUT" "moved the binary"
    assert_not_contains "$OUT" "links to it"

    test_case "and the receipt, the completion and the PATH check still happen ($kept_case)"
    assert_equals "$HM_SYSTEM_BIN/hm" "$(sed -n 's/^path=//p' "$HOME/.hm/install-receipt")"
    assert_equals "" "$(sed -n 's/^link=//p' "$HOME/.hm/install-receipt")"
    assert_equals "# hm zsh completion of $kept_version" "$(cat "$HOME/.hm/completion/hm.zsh")"
    assert_contains "$OUT" "$HM_SYSTEM_BIN is not in your PATH"
done

new_case rc-binary-home-binary-restored
publish 2.0.0
publish 2.1.0
api_releases 2.0.0
run_install
# A ~/.hm/bin/hm already there: an earlier hm 2 that is not the one in the system directory
mkdir -p "$HOME/.hm/bin"
{ cat "$FAKE_RELEASES/2.0.0/hm_linux_amd64"; echo "# mine"; } >"$HOME/.hm/bin/hm"
chmod 755 "$HOME/.hm/bin/hm"
home_binary_before=$(sha256_of "$HOME/.hm/bin/hm")
chmod u-w "$HM_SYSTEM_BIN"
api_releases 2.1.0 2.0.0
run_install_refusing_sudo
chmod u+w "$HM_SYSTEM_BIN"

test_case "a refused sudo puts back the ~/.hm/bin/hm that was there, so nothing was changed"
if [ "$(id -u)" -eq 0 ]; then
    echo "  - skipped: running as root, a read-only folder does not need sudo"
else
    assert_equals "1" "$STATUS"
    assert_contains "$OUT" "Nothing was changed"
    assert_equals "$home_binary_before" "$(sha256_of "$HOME/.hm/bin/hm")"
    assert_equals "-rwxr-xr-x" "$(ls -l "$HOME/.hm/bin/hm" | cut -c1-10)"
    assert_equals "hm" "$(ls -A "$HOME/.hm/bin")"
    assert_equals "no" "$([ -L "$HM_SYSTEM_BIN/hm" ] && echo yes || echo no)"
    assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
fi

new_case receipt-without-link
publish 2.0.0
api_releases 2.0.0
run_install
# What hm update leaves after moving the binary: ~/.hm/bin/hm, a link in its place, the receipt untouched
mkdir -p "$HOME/.hm/bin"
mv "$HM_SYSTEM_BIN/hm" "$HOME/.hm/bin/hm"
ln -s "$HOME/.hm/bin/hm" "$HM_SYSTEM_BIN/hm"
chmod u-w "$HM_SYSTEM_BIN"
publish 2.1.0
api_releases 2.1.0 2.0.0
run_install
chmod u+w "$HM_SYSTEM_BIN"

test_case "a receipt without link=, whose path is now a link to ~/.hm/bin/hm, still upgrades, without sudo"
assert_equals "0" "$STATUS"
assert_equals "hm 2.1.0" "$("$HOME/.hm/bin/hm" --version | head -n 1)"
assert_equals "$HOME/.hm/bin/hm" "$(readlink "$HM_SYSTEM_BIN/hm")"
assert_contains "$OUT" "upgraded hm 2.0.0 to 2.1.0"
assert_equals "" "$(cat "$FAKE_SUDO_LOG")"
assert_equals "$HM_SYSTEM_BIN/hm" "$(sed -n 's/^link=//p' "$HOME/.hm/install-receipt")"

new_case layout-link-dangling
publish 2.0.0
api_releases 2.0.0
ln -s "$HOME/.hm/bin/hm" "$HM_SYSTEM_BIN/hm"
run_install

test_case "a link to a missing ~/.hm/bin/hm is never a source checkout: hm is put back behind it"
assert_equals "0" "$STATUS"
assert_equals "$HOME/.hm/bin/hm" "$(readlink "$HM_SYSTEM_BIN/hm")"
assert_equals "hm 2.0.0" "$("$HOME/.hm/bin/hm" --version | head -n 1)"
assert_not_contains "$OUT" "source checkout"
assert_not_contains "$OUT" "1.x"
assert_not_contains "$OUT" "rm -rf"

new_case layout-link-uninstall
ln -s "$HOME/.hm/bin/hm" "$HM_SYSTEM_BIN/hm"
run_install --uninstall

test_case "--uninstall removes a link to a missing ~/.hm/bin/hm, and does not call it a checkout"
assert_equals "0" "$STATUS"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"
assert_not_contains "$OUT" "source checkout"
assert_not_contains "$OUT" "1.x"

new_case custom-dir
publish 2.0.0
api_releases 2.0.0
HM_INSTALL_DIR="$CASE/opt/bin" run_install

test_case "HM_INSTALL_DIR chooses the directory"
assert_equals "0" "$STATUS"
assert_equals "yes" "$(exists "$CASE/opt/bin/hm")"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"

test_case "a directory of the user's own gets the binary itself: no link, no ~/.hm/bin"
assert_equals "no" "$([ -L "$CASE/opt/bin/hm" ] && echo yes || echo no)"
assert_equals "no" "$(exists "$HOME/.hm/bin")"
assert_equals "" "$(sed -n 's/^link=//p' "$HOME/.hm/install-receipt")"

new_case explicit-dir-not-writable
publish 2.0.0
api_releases 2.0.0
chmod u-w "$HM_SYSTEM_BIN"
run_install --dir "$HM_SYSTEM_BIN"
chmod u+w "$HM_SYSTEM_BIN"

test_case "a --dir that is not writable gets the binary itself, written with sudo: no link, no ~/.hm/bin"
assert_equals "0" "$STATUS"
assert_equals "no" "$([ -L "$HM_SYSTEM_BIN/hm" ] && echo yes || echo no)"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_equals "no" "$(exists "$HOME/.hm/bin")"
assert_contains "$(cat "$FAKE_SUDO_LOG")" "cp "
assert_equals "$HM_SYSTEM_BIN/hm" "$(sed -n 's/^path=//p' "$HOME/.hm/install-receipt")"
assert_equals "" "$(sed -n 's/^link=//p' "$HOME/.hm/install-receipt")"

new_case explicit-dir-sudo-refused
publish 2.0.0
api_releases 2.0.0
chmod u-w "$HM_SYSTEM_BIN"
HM_INSTALL_DIR="$HM_SYSTEM_BIN" run_install_refusing_sudo
chmod u+w "$HM_SYSTEM_BIN"

test_case "an HM_INSTALL_DIR whose sudo is refused is refused too: nothing changed, no fallback to ~/.local/bin"
if [ "$(id -u)" -eq 0 ]; then
    echo "  - skipped: running as root, a read-only folder does not need sudo"
else
    assert_equals "1" "$STATUS"
    assert_contains "$OUT" "Nothing was changed"
    assert_equals "" "$(ls -A "$HM_SYSTEM_BIN")"
    assert_equals "no" "$(exists "$HOME/.local/bin/hm")"
    assert_equals "no" "$(exists "$HOME/.hm/bin")"
    assert_equals "no" "$(exists "$HOME/.hm/install-receipt")"
fi

new_case foreign
publish 2.0.0
api_releases 2.0.0
printf '#!/bin/sh\necho something else\n' >"$HM_SYSTEM_BIN/hm"
chmod +x "$HM_SYSTEM_BIN/hm"
run_install --dir "$HM_SYSTEM_BIN"

test_case "an hm that is not hm is never overwritten"
assert_equals "1" "$STATUS"
assert_contains "$OUT" "is not an hm this installer can replace"
assert_equals "something else" "$("$HM_SYSTEM_BIN/hm")"

new_case foreign-link
publish 2.0.0
api_releases 2.0.0
ln -s /opt/elsewhere/hm "$HM_SYSTEM_BIN/hm"
run_install

test_case "a link somebody else made is left alone; hm goes to ~/.local/bin instead"
assert_equals "0" "$STATUS"
assert_equals "/opt/elsewhere/hm" "$(readlink "$HM_SYSTEM_BIN/hm")"
assert_equals "hm 2.0.0" "$("$HOME/.local/bin/hm" --version | head -n 1)"

new_case sudo-refused
publish 2.0.0
api_releases 2.0.0
mkdir -p "$HOME/hm/bin"
printf '#!/bin/sh\n' >"$HOME/hm/bin/run"
ln -s "$HOME/hm/bin/run" "$HM_SYSTEM_BIN/hm"
chmod u-w "$HM_SYSTEM_BIN"
mkdir -p "$CASE/no-sudo"
printf '#!/bin/sh\necho "sudo $*" >>"$FAKE_SUDO_LOG"\nexit 1\n' >"$CASE/no-sudo/sudo"
chmod +x "$CASE/no-sudo/sudo"
(PATH="$CASE/no-sudo:$TOOLS:$BASE_PATH" bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
OUT=$(cat "$CASE/out" "$CASE/err")
chmod u+w "$HM_SYSTEM_BIN"

test_case "a refused sudo changes nothing and claims nothing"
if [ "$(id -u)" -eq 0 ]; then
    echo "  - skipped: running as root, a read-only folder does not need sudo"
else
    assert_equals "1" "$STATUS"
    assert_contains "$OUT" "Could not write $HM_SYSTEM_BIN/hm"
    assert_not_contains "$OUT" "installed at"
    assert_equals "$HOME/hm/bin/run" "$(readlink "$HM_SYSTEM_BIN/hm")"
    assert_equals "hm" "$(ls -A "$HM_SYSTEM_BIN")"
    assert_equals "no" "$(exists "$HOME/.hm/bin/hm")"
fi

# ------------------------------------------------------------------ migrating from 1.x

new_case migrate
publish 2.0.0
api_releases 2.0.0
mkdir -p "$HOME/hm/bin" "$HOME/hm/console"
printf '#!/usr/bin/env bash\necho shell-launcher\n' >"$HOME/hm/bin/run"
printf 'complete -W "start stop" hm\n' >"$HOME/hm/console/hm-completion.bash"
chmod +x "$HOME/hm/bin/run"
git -C "$HOME/hm" init -q
git -C "$HOME/hm" add -A
git -C "$HOME/hm" -c user.email=t@t -c user.name=t commit -qm 1.5.1
echo "# a local tweak" >>"$HOME/hm/bin/run"
ln -s "$HOME/hm/bin/run" "$HM_SYSTEM_BIN/hm"
printf 'source %s/console/hm-completion.bash\n' "$HOME/hm" >>"$HOME/.zshrc"
printf 'source %s/console/hm-completion.bash\n' "$HOME/hm" >"$HOME/.bashrc"
chmod u-w "$HM_SYSTEM_BIN"
before=$(cd "$HOME/hm" && find . -path ./.git -prune -o -type f -print | sort | xargs cat | sha256_of /dev/stdin)
run_install
chmod u+w "$HM_SYSTEM_BIN"

test_case "the 1.x link is replaced, at the same path, by a link to the binary in ~/.hm/bin"
assert_equals "0" "$STATUS"
assert_equals "$HOME/.hm/bin/hm" "$(readlink "$HM_SYSTEM_BIN/hm")"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_equals "no" "$(exists "$HOME/.local/bin/hm")"
assert_contains "$OUT" "replaced the link $HM_SYSTEM_BIN/hm -> $HOME/hm/bin/run of the git-clone installation with a link to $HOME/.hm/bin/hm"

test_case "with sudo, only for that directory, because it was not writable"
assert_contains "$(cat "$FAKE_SUDO_LOG")" "mv -f $HM_SYSTEM_BIN/.hm.install"
assert_equals "" "$(sudo_outside_system)"

test_case "the 1.x completion line is replaced by the new one, in every profile"
assert_not_contains "$(cat "$HOME/.zshrc" "$HOME/.bashrc")" "console/hm-completion.bash"
assert_equals "1" "$(grep -c 'hm shell completion' "$HOME/.zshrc")"
assert_contains "$OUT" "removed the 1.x completion line from $HOME/.zshrc"

test_case "~/hm is left untouched"
assert_equals "$before" "$(cd "$HOME/hm" && find . -path ./.git -prune -o -type f -print | sort | xargs cat | sha256_of /dev/stdin)"

test_case "and the notice says it is unused, has local changes, and how to delete it"
assert_contains "$OUT" "The 1.x installation at $HOME/hm is no longer used"
assert_contains "$OUT" "It has local changes (1 changed files in git status)"
assert_contains "$OUT" "rm -rf $HOME/hm"

# ------------------------------------------------------------------ refusals

new_case tampered
publish 2.0.0
api_releases 2.0.0
printf 'tampered\n' >>"$FAKE_RELEASES/2.0.0/hm_linux_amd64"
run_install

test_case "a checksum mismatch installs nothing"
assert_equals "1" "$STATUS"
assert_contains "$OUT" "does not match the checksum"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"
assert_equals "no" "$(exists "$HOME/.hm/completion/hm.zsh")"
assert_equals "" "$(ls -A "$TMPDIR")"

new_case shell-version
publish 2.0.0
run_install --version 1.5.1

test_case "a version before 2.0 is refused before anything is downloaded"
assert_equals "2" "$STATUS"
assert_contains "$OUT" "'1.5.1' is a version of the shell tool (1.x)"
assert_contains "$OUT" "git clone -b 1.5.1"
assert_equals "" "$(cat "$FAKE_CURL_LOG")"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"

HM_VERSION=1.6.0 run_install

test_case "HM_VERSION is refused the same way"
assert_equals "2" "$STATUS"
assert_contains "$OUT" "'1.6.0' is a version of the shell tool (1.x)"

new_case no-release
publish 1.6.0
api_releases 1.6.0 1.5.1
run_install

test_case "with no release of 2.0 or later it says so and installs nothing"
assert_equals "1" "$STATUS"
assert_contains "$OUT" "No release of hm 2.0 or later has been published yet"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"

api_releases 2.0.0-rc.1 1.6.0
run_install

test_case "and when there are only pre-releases, how to install one"
assert_equals "1" "$STATUS"
assert_contains "$OUT" "the latest pre-release is 2.0.0-rc.1. Install it with --pre"

new_case token
publish 2.0.0
api_releases 2.0.0
GITHUB_TOKEN=secret-token run_install

test_case "GITHUB_TOKEN is sent to the API and nowhere else"
assert_contains "$(grep api.github.com "$FAKE_CURL_LOG")" "Authorization: Bearer secret-token"
assert_not_contains "$(grep -v api.github.com "$FAKE_CURL_LOG")" "secret-token"

# ------------------------------------------------------------------ uninstall

new_case uninstall
publish 2.0.0
api_releases 2.0.0
mkdir -p "$HOME/.hm/cache"
run_install
run_install --uninstall

test_case "--uninstall removes the binary, the completion and its profile line"
assert_equals "0" "$STATUS"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"
assert_equals "no" "$(exists "$HOME/.hm/completion")"
assert_not_contains "$(cat "$HOME/.zshrc")" "hm shell completion"
assert_contains "$(cat "$HOME/.zshrc")" "export FOO=1"

test_case "and says how to remove hm's state, which it keeps"
assert_equals "yes" "$(exists "$HOME/.hm/cache")"
assert_contains "$OUT" "rm -rf $HOME/.hm"

new_case uninstall-foreign
printf '#!/bin/sh\necho something else\n' >"$HM_SYSTEM_BIN/hm"
chmod +x "$HM_SYSTEM_BIN/hm"
run_install --uninstall

test_case "--uninstall never removes an hm it did not install"
assert_equals "0" "$STATUS"
assert_equals "yes" "$(exists "$HM_SYSTEM_BIN/hm")"
assert_contains "$OUT" "no binary was removed"

# ------------------------------------------------------------------ the profile is never lost

LINE='[ -r "$HOME/.hm/completion/hm.zsh" ] && . "$HOME/.hm/completion/hm.zsh" # hm shell completion'
# A profile with what a careless rewrite mangles: tabs, trailing spaces, backslashes, percent
# signs, a carriage return, blank lines and a line that looks like the mark but is not it
weird_profile() {
    printf '# my profile\n\texport FOO="a b"  \nalias x='"'"'printf "%%s\\n"'"'"'\r\n\n\n# hm completion, not the mark\nexport BAR=\\\\$HOME\n' >"$1"
}

new_case profile-bytes
publish 2.0.0
api_releases 2.0.0
weird_profile "$HOME/.zshrc"
chmod 600 "$HOME/.zshrc"
cp "$HOME/.zshrc" "$CASE/original"
run_install

test_case "the profile keeps every byte, with only the completion line added"
assert_equals "0" "$STATUS"
cat "$CASE/original" >"$CASE/expected"
printf '%s\n' "$LINE" >>"$CASE/expected"
assert_equals "same" "$(cmp -s "$CASE/expected" "$HOME/.zshrc" && echo same || echo different)"

test_case "and its permissions, with the original kept once in .zshrc.hm-backup"
assert_equals "-rw-------" "$(ls -l "$HOME/.zshrc" | cut -c1-10)"
assert_equals "same" "$(cmp -s "$CASE/original" "$HOME/.zshrc.hm-backup" && echo same || echo different)"

test_case "and no temporary file is left beside it"
assert_equals "" "$(cd "$HOME" && ls -A | grep '\.hm-' | grep -v '^\.zshrc\.hm-backup$')"

run_install --uninstall

test_case "uninstalling takes the one line out again, byte for byte"
assert_equals "same" "$(cmp -s "$CASE/original" "$HOME/.zshrc" && echo same || echo different)"
assert_equals "same" "$(cmp -s "$CASE/original" "$HOME/.zshrc.hm-backup" && echo same || echo different)"

new_case profile-link
publish 2.0.0
api_releases 2.0.0
mkdir -p "$CASE/dotfiles"
rm -f "$HOME/.zshrc"
weird_profile "$CASE/dotfiles/zshrc"
cp "$CASE/dotfiles/zshrc" "$CASE/original"
ln -s ../dotfiles/zshrc "$HOME/.zshrc"
run_install

test_case "a profile that is a link stays the same link, and the file it points to is edited"
assert_equals "0" "$STATUS"
assert_equals "../dotfiles/zshrc" "$(readlink "$HOME/.zshrc")"
cat "$CASE/original" >"$CASE/expected"
printf '%s\n' "$LINE" >>"$CASE/expected"
assert_equals "same" "$(cmp -s "$CASE/expected" "$CASE/dotfiles/zshrc" && echo same || echo different)"
assert_equals "zshrc" "$(ls -A "$CASE/dotfiles")"

new_case profile-dangling
publish 2.0.0
api_releases 2.0.0
rm -f "$HOME/.zshrc"
ln -s "$CASE/nowhere/zshrc" "$HOME/.zshrc"
run_install

test_case "a link to nothing is left alone, with the line to add printed"
assert_equals "0" "$STATUS"
assert_equals "$CASE/nowhere/zshrc" "$(readlink "$HOME/.zshrc")"
assert_equals "no" "$(exists "$CASE/nowhere")"
assert_contains "$OUT" "Add this line to it yourself"
assert_contains "$OUT" "$LINE"

new_case profile-read-only
publish 2.0.0
api_releases 2.0.0
weird_profile "$HOME/.zshrc"
chmod 444 "$HOME/.zshrc"
cp "$HOME/.zshrc" "$CASE/original"
run_install

test_case "a read-only profile is not touched; the install still succeeds and says what to add"
assert_equals "0" "$STATUS"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_equals "same" "$(cmp -s "$CASE/original" "$HOME/.zshrc" && echo same || echo different)"
assert_equals "-r--r--r--" "$(ls -l "$HOME/.zshrc" | cut -c1-10)"
assert_contains "$OUT" "is not writable"
assert_contains "$OUT" "$LINE"
assert_contains "$OUT" "not yet sourced from $HOME/.zshrc"
chmod 644 "$HOME/.zshrc"

new_case profile-mv-fails
publish 2.0.0
api_releases 2.0.0
weird_profile "$HOME/.zshrc"
cp "$HOME/.zshrc" "$CASE/original"
mkdir -p "$CASE/broken-mv"
# mv: refuses the one rename that would put the new profile in place, as a full disk would
printf '#!/bin/sh\ncase "$*" in *.zshrc.hm-*) exit 1 ;; esac\nexec /bin/mv "$@"\n' >"$CASE/broken-mv/mv"
chmod +x "$CASE/broken-mv/mv"
(PATH="$CASE/broken-mv:$TOOLS:$BASE_PATH" bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
OUT=$(cat "$CASE/out" "$CASE/err")

test_case "a failure halfway leaves the profile exactly as it was, and no temporary file"
assert_equals "0" "$STATUS"
assert_equals "same" "$(cmp -s "$CASE/original" "$HOME/.zshrc" && echo same || echo different)"
assert_equals "" "$(cd "$HOME" && ls -A | grep '\.hm-' | grep -v '^\.zshrc\.hm-backup$')"
assert_contains "$OUT" "it was left exactly as it was"
assert_contains "$OUT" "$LINE"

new_case profile-no-final-newline
publish 2.0.0
api_releases 2.0.0
printf '# my profile\nexport FOO=1' >"$HOME/.zshrc"
run_install

test_case "a profile without a final newline gets the line on a line of its own, nothing else changed"
assert_equals "0" "$STATUS"
printf '# my profile\nexport FOO=1\n%s\n' "$LINE" >"$CASE/expected"
assert_equals "same" "$(cmp -s "$CASE/expected" "$HOME/.zshrc" && echo same || echo different)"

new_case profile-hard-link
publish 2.0.0
api_releases 2.0.0
weird_profile "$CASE/zshrc-shared"
cp "$CASE/zshrc-shared" "$CASE/original"
rm -f "$HOME/.zshrc"
ln "$CASE/zshrc-shared" "$HOME/.zshrc"
run_install

test_case "a profile with another hard link is not replaced, which would split the two; it says what to add"
assert_equals "0" "$STATUS"
assert_equals "same" "$(cmp -s "$CASE/original" "$HOME/.zshrc" && echo same || echo different)"
assert_equals "2" "$(ls -ln "$HOME/.zshrc" | awk '{ print $2 }')"
assert_contains "$OUT" "it was left exactly as it was"
assert_contains "$OUT" "$LINE"
assert_equals "" "$(cd "$HOME" && ls -A | grep '\.hm-')"

new_case profile-other-owner
publish 2.0.0
api_releases 2.0.0
weird_profile "$HOME/.zshrc"
cp "$HOME/.zshrc" "$CASE/original"
mkdir -p "$CASE/other-user"
# id: another user running the installer, for whom this writable profile belongs to someone else
printf '#!/bin/sh\n[ "$1" = "-u" ] && { echo 4242; exit 0; }\nexec /usr/bin/id "$@"\n' >"$CASE/other-user/id"
chmod +x "$CASE/other-user/id"
(PATH="$CASE/other-user:$TOOLS:$BASE_PATH" bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
OUT=$(cat "$CASE/out" "$CASE/err")

test_case "a profile owned by another user is not replaced by one owned by this user; it says what to add"
assert_equals "0" "$STATUS"
assert_equals "same" "$(cmp -s "$CASE/original" "$HOME/.zshrc" && echo same || echo different)"
assert_contains "$OUT" "it was left exactly as it was"
assert_contains "$OUT" "$LINE"
assert_equals "" "$(cd "$HOME" && ls -A | grep '\.hm-')"

# ------------------------------------------------------------------ the token is never on argv

new_case token-argv
publish 2.0.0
api_releases 2.0.0
GITHUB_TOKEN=secret-token run_install --pre

test_case "GITHUB_TOKEN reaches curl through a file only this user can read, never its arguments"
assert_equals "0" "$STATUS"
assert_contains "$(grep api.github.com "$FAKE_CURL_LOG")" "Authorization: Bearer secret-token"
assert_not_contains "$(cat "$FAKE_CURL_ARGV")" "secret-token"
assert_equals "-rw-------" "$(head -n 1 "$FAKE_CURL_MODES")"
assert_equals "" "$(ls -A "$TMPDIR")"

# ------------------------------------------------------------------ the API only when needed

new_case latest-redirect
publish 2.1.0
api_releases 2.0.0
export FAKE_LATEST=2.1.0
run_install

test_case "the latest final release comes from the releases/latest redirect, without the API"
assert_equals "0" "$STATUS"
assert_equals "hm 2.1.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_not_contains "$(cat "$FAKE_CURL_LOG")" "api.github.com"

new_case latest-redirect-shell
publish 2.0.0
api_releases 2.0.0 1.6.1
export FAKE_LATEST=1.6.1
run_install

test_case "a latest release of 1.x falls back to the API, which finds the newest 2.x"
assert_equals "0" "$STATUS"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_contains "$(cat "$FAKE_CURL_LOG")" "api.github.com"

# A machine with wget and no curl: wget prints the redirect among the server's headers on stderr and
# exits 8 because it was told to follow none, as GNU wget does
WGET_TOOLS="$LAB/wget-tools"
NO_CURL="$LAB/no-curl"
mkdir -p "$WGET_TOOLS" "$NO_CURL"
for tool in /usr/bin/* /bin/*; do
    case "$(basename "$tool")" in curl | wget) continue ;; esac
    [ -e "$NO_CURL/$(basename "$tool")" ] || ln -s "$tool" "$NO_CURL/$(basename "$tool")"
done
cp "$TOOLS/uname" "$TOOLS/sudo" "$TOOLS/jq" "$WGET_TOOLS/"
cat >"$WGET_TOOLS/wget" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_CURL_ARGV"
# GNU wget 1.16 and later list --show-progress in their help; busybox's does not
[ "$1" == "--help" ] && { echo "       --show-progress             display the progress bar in any verbosity mode"; exit 0; }
out="" url="" headers=false sent=""
for argument in "$@"; do
    case "$argument" in
        -S) headers=true ;;
        # The header a wgetrc of its own carries is logged with the URL, as curl's -H is
        --config=*) sent="$sent $(sed -n 's/^header = //p' "${argument#--config=}")" ;;
        -*) ;;
        *) [ "$previous" == "-O" ] && out="$argument" || url="$argument" ;;
    esac
    previous="$argument"
done
echo "$url$sent" >>"$FAKE_CURL_LOG"
case "$url" in
    https://github.com/*/releases/latest)
        if $headers && [ -n "${FAKE_LATEST:-}" ]; then
            printf '  HTTP/1.1 302 Found\r\n  Location: %s\r\n' "${url%/latest}/tag/$FAKE_LATEST" >&2
            echo "0 redirections exceeded." >&2
        fi
        exit 8 ;;
    https://api.github.com/*) source="$FAKE_API" ;;
    https://github.com/*/releases/download/*) source="$FAKE_RELEASES/${url#*/releases/download/}" ;;
    *) exit 8 ;;
esac
[ -f "$source" ] || exit 8
cp "$source" "$out"
EOF
chmod +x "$WGET_TOOLS"/*

new_case latest-redirect-wget
publish 2.1.0
api_releases 2.0.0
export FAKE_LATEST=2.1.0
(PATH="$WGET_TOOLS:$NO_CURL" bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?

test_case "with wget and no curl, the latest final release still comes from the redirect, without the API"
assert_equals "0" "$STATUS"
assert_equals "hm 2.1.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_not_contains "$(cat "$FAKE_CURL_LOG")" "api.github.com"

test_case "and without a terminal wget stays quiet: no progress bar"
assert_contains "$(grep hm_linux_amd64 "$FAKE_CURL_ARGV")" "-q"
assert_not_contains "$(cat "$FAKE_CURL_ARGV")" "progress"

new_case latest-redirect-wget-styled
publish 2.1.0
api_releases 2.0.0
export FAKE_LATEST=2.1.0
(PATH="$WGET_TOOLS:$NO_CURL" CLICOLOR_FORCE=1 bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
PLAIN=$(cat "$CASE/out" "$CASE/err" | sed $'s/\033\\[[0-9;]*m//g')

test_case "styled, with wget only, the binary's download is hm's own, wget silent; the checksums draw nothing"
assert_equals "0" "$STATUS"
assert_contains "$PLAIN" "✓ Latest release: 2.1.0"
assert_not_contains "$(cat "$FAKE_CURL_ARGV")" "progress"
assert_contains "$(grep hm_linux_amd64 "$FAKE_CURL_ARGV")" "-S"
assert_not_contains "$(grep checksums.txt "$FAKE_CURL_ARGV")" "-S"
assert_contains "$PLAIN" "Downloading hm_linux_amd64…  $(wc -c <"$FAKE_RELEASES/2.1.0/hm_linux_amd64" | tr -d ' ') B"
assert_contains "$PLAIN" "✓ Downloaded hm 2.1.0 for linux/amd64, checksum verified"

new_case rate-limited
publish 2.0.0
export FAKE_API_STATUS=403
run_install --pre

test_case "a rate-limited API says so, with how to get past it"
assert_equals "1" "$STATUS"
assert_contains "$OUT" "rate limit was reached (HTTP 403)"
assert_contains "$OUT" "GITHUB_TOKEN"
assert_contains "$OUT" "--version X.Y.Z"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"

# ------------------------------------------------------------------ re-running finds the same hm

new_case rerun-custom-dir
publish 2.0.0
api_releases 2.0.0
run_install --dir "$CASE/opt/bin"
publish 2.1.0
api_releases 2.1.0 2.0.0
run_install

test_case "running the installer again upgrades the hm it installed, wherever --dir put it"
assert_equals "0" "$STATUS"
assert_equals "hm 2.1.0" "$("$CASE/opt/bin/hm" --version | head -n 1)"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"
assert_contains "$OUT" "upgraded hm 2.0.0 to 2.1.0"

# ------------------------------------------------------------------ a branch

new_case branch
run_install --version main

test_case "a branch is refused with how to build it from source"
assert_equals "2" "$STATUS"
assert_contains "$OUT" "'main' is not a release version"
assert_contains "$OUT" "git clone -b main"
assert_contains "$OUT" "go build -o hm ./cmd/hm"
assert_equals "" "$(cat "$FAKE_CURL_LOG")"

# ------------------------------------------------------------------ 1.x install settings

new_case settings
publish 2.0.0
api_releases 2.0.0
mkdir -p "$HOME/hm/bin" "$HOME/hm/data"
printf '#!/bin/sh\n' >"$HOME/hm/bin/run"
printf '{\n  "currency": "USD"\n}\n' >"$HOME/hm/data/config.json"
git -C "$HOME/hm" init -q
ln -s "$HOME/hm/bin/run" "$HM_SYSTEM_BIN/hm"
run_install

test_case "a 1.x installation's install settings are copied to ~/.hm/config.json"
assert_equals "0" "$STATUS"
assert_equals "same" "$(cmp -s "$HOME/hm/data/config.json" "$HOME/.hm/config.json" && echo same || echo different)"
assert_contains "$OUT" "copied the 1.x install settings"

printf '{"currency": "EUR"}\n' >"$HOME/.hm/config.json"
run_install

test_case "and never over settings already there"
assert_equals '{"currency": "EUR"}' "$(cat "$HOME/.hm/config.json")"

test_case "the copy is readable by this user only, since it can hold the admin password"
assert_equals "-rw-------" "$(rm -f "$HOME/.hm/config.json"; run_install; ls -l "$HOME/.hm/config.json" | cut -c1-10)"

# settings_case <content>: a 1.x installation of its own whose data/config.json holds content
SETTINGS_CASES=0
settings_case() {
    SETTINGS_CASES=$((SETTINGS_CASES + 1))
    new_case "settings-$SETTINGS_CASES"
    publish 2.0.0
    api_releases 2.0.0
    mkdir -p "$HOME/hm/bin" "$HOME/hm/data"
    printf '#!/bin/sh\n' >"$HOME/hm/bin/run"
    printf '%s' "$1" >"$HOME/hm/data/config.json"
    git -C "$HOME/hm" init -q
    ln -s "$HOME/hm/bin/run" "$HM_SYSTEM_BIN/hm"
    run_install
}

# The same JSON objects hm itself accepts (dockergento's copyLegacyInstallSettings), the same refused
for settings in '{"a": {"b": [1, 2.5e3, true, null]}, "c": "} \" \\\\ é {"}' $'{\n\t"admin-password": "x"\r\n}\n' '{}'; do
    settings_case "$settings"
    test_case "a valid JSON object is copied: $settings"
    assert_equals "yes" "$(exists "$HOME/.hm/config.json")"
done
for settings in '{"currency": }' '{"a": 1}{"b": 2}' '{"a": 1,}' "{'a': 1}" '{"a": 01}' '{"a": "x' '[1]' '"text"' '{"a" 1}' ''; do
    settings_case "$settings"
    test_case "what is not a valid JSON object is not copied: $settings"
    assert_equals "0" "$STATUS"
    assert_equals "no" "$(exists "$HOME/.hm/config.json")"
    assert_contains "$OUT" "not a JSON object"
done

new_case settings-link
publish 2.0.0
api_releases 2.0.0
mkdir -p "$HOME/hm/bin" "$HOME/hm/data" "$CASE/elsewhere"
printf '#!/bin/sh\n' >"$HOME/hm/bin/run"
printf '{"secret": "of another file"}\n' >"$CASE/elsewhere/config.json"
ln -s "$CASE/elsewhere/config.json" "$HOME/hm/data/config.json"
git -C "$HOME/hm" init -q
ln -s "$HOME/hm/bin/run" "$HM_SYSTEM_BIN/hm"
run_install

test_case "a config.json that is a link is not followed out of the 1.x installation"
assert_equals "0" "$STATUS"
assert_equals "no" "$(exists "$HOME/.hm/config.json")"

# ------------------------------------------------------------------ a contributor's checkout
new_case source-checkout
publish 2.0.0
api_releases 2.0.0
mkdir -p "$HOME/hm/bin" "$HOME/hm/data"
printf '#!/bin/sh\necho "hm 2.1.0-dev"\n' >"$HOME/hm/bin/hm"
chmod +x "$HOME/hm/bin/hm"
printf '{"from": "the checkout"}\n' >"$HOME/hm/data/config.json"
git -C "$HOME/hm" init -q
echo "work in progress" >"$HOME/hm/notes.txt"
ln -s "$HOME/hm/bin/hm" "$HM_SYSTEM_BIN/hm"
run_install

test_case "a link to a build of a 2.x source checkout is replaced by the binary"
assert_equals "0" "$STATUS"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_contains "$OUT" "replaced the link $HM_SYSTEM_BIN/hm -> $HOME/hm/bin/hm, a build of the source checkout $HOME/hm, with the binary"
assert_contains "$OUT" "to run your build again: ln -sf $HOME/hm/bin/hm $HM_SYSTEM_BIN/hm"

test_case "and the checkout is neither called 1.x nor offered for deletion, nor its settings copied"
assert_not_contains "$OUT" "1.x"
assert_not_contains "$OUT" "rm -rf"
assert_not_contains "$OUT" "local changes"
assert_equals "no" "$(exists "$HOME/.hm/config.json")"
assert_equals "work in progress" "$(cat "$HOME/hm/notes.txt")"

new_case source-checkout-legacy
publish 2.0.0
api_releases 2.0.0
mkdir -p "$HOME/hm/bin" "$HOME/hm/console"
printf '#!/bin/sh\necho "hm 1.6.0"\n' >"$HOME/hm/bin/hm"
printf '#!/bin/sh\n' >"$HOME/hm/bin/run"
chmod +x "$HOME/hm/bin/hm" "$HOME/hm/bin/run"
git -C "$HOME/hm" init -q
ln -s "$HOME/hm/bin/hm" "$HM_SYSTEM_BIN/hm"
run_install

test_case "a link to bin/hm of a checkout that still has bin/run is a 1.x installation"
assert_equals "0" "$STATUS"
assert_contains "$OUT" "of the git-clone installation with the binary"
assert_contains "$OUT" "The 1.x installation at $HOME/hm is no longer used"

# ------------------------------------------------------------------ how it looks
new_case styled
publish 2.0.0
api_releases 2.0.0
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
OUT=$(cat "$CASE/out" "$CASE/err")

PLAIN=$(printf '%s' "$OUT" | sed $'s/\033\\[[0-9;]*m//g')

test_case "with colours forced, a header, a tick per step and a closing box"
assert_equals "0" "$STATUS"
assert_contains "$OUT" $'\033['
assert_contains "$PLAIN" "hiberus Dockergento · installer"
assert_contains "$PLAIN" "✓ Downloaded hm 2.0.0 for linux/amd64, checksum verified"
assert_contains "$PLAIN" "✓ Installed at $HM_SYSTEM_BIN/hm"
assert_contains "$PLAIN" "✓ Shell completion: $HOME/.hm/completion/hm.zsh"
assert_contains "$PLAIN" "  ! $HM_SYSTEM_BIN is not in your PATH"
assert_contains "$PLAIN" "│  hm 2.0.0 is ready"
assert_contains "$PLAIN" "Next: open a new terminal"
assert_not_contains "$PLAIN" "  - "

test_case "and every line of the box is as wide as its border"
widths=$(printf '%s\n' "$PLAIN" | grep -E '^  [╭│╰]' | while IFS= read -r line; do printf '%s' "$line" | sed 's/╭/x/g; s/╮/x/g; s/╰/x/g; s/╯/x/g; s/│/x/g; s/─/x/g' | wc -c | tr -d ' '; done | sort -u)
assert_equals "1" "$(printf '%s\n' "$widths" | wc -l | tr -d ' ')"

test_case "a spinner on stderr while the latest release is looked up, then a tick naming it"
assert_contains "$(cat "$CASE/err")" "Looking up the latest release…"
assert_contains "$PLAIN" "✓ Latest release: 2.0.0"
assert_not_contains "$(cat "$CASE/out")" "Looking up"

ASSET_SIZE=$(wc -c <"$FAKE_RELEASES/2.0.0/hm_linux_amd64" | tr -d ' ')

test_case "the binary's download is hm's own bar on stderr, curl silent; the checksums and the API draw nothing"
assert_contains "$(grep hm_linux_amd64 "$FAKE_CURL_ARGV")" "-fsSL"
assert_not_contains "$(cat "$FAKE_CURL_ARGV")" "progress"
assert_not_contains "$(cat "$CASE/err")" "####"
assert_not_contains "$(cat "$CASE/err")" "100.0%"
assert_contains "$(sed $'s/\033\\[[0-9;]*m//g' "$CASE/err")" "████████████████████████  100%  $ASSET_SIZE B / $ASSET_SIZE B"
assert_equals "1" "$(grep -c 'checksums.txt' "$FAKE_CURL_ARGV")"
assert_not_contains "$(grep -v hm_linux_amd64 "$FAKE_CURL_ARGV")" " -D "

test_case "and the bar leaves nothing behind: its last frame is cleared before the tick that follows"
assert_contains "$(sed $'s/\033\\[[0-9;]*m//g' "$CASE/err")" "$ASSET_SIZE B / $ASSET_SIZE B"$'\033[K\r\033[K'
assert_not_contains "$(cat "$CASE/out")" "████"

# err_settles <file>: whether nothing writes to the file any more once the installer has exited,
# which a spinner left running would
err_settles() {
    local before
    before=$(wc -c <"$1")
    sleep 0.3
    [ "$before" == "$(wc -c <"$1")" ] && echo yes || echo no
}

test_case "and the spinner is gone once the installer exits"
assert_equals "yes" "$(err_settles "$CASE/err")"

new_case styled-download-growing
publish 2.0.0
export FAKE_DOWNLOAD_HALVES=1
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$INSTALL_SH" --version 2.0.0 >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
HALF=$((ASSET_SIZE / 2))

test_case "styled, the bar follows the file as it grows against the length the server announced"
assert_equals "0" "$STATUS"
assert_contains "$(sed $'s/\033\\[[0-9;]*m//g' "$CASE/err")" "$((HALF * 100 / ASSET_SIZE))%  $HALF B / $ASSET_SIZE B"
assert_contains "$(sed $'s/\033\\[[0-9;]*m//g' "$CASE/err")" "100%  $ASSET_SIZE B / $ASSET_SIZE B"
assert_equals "yes" "$(err_settles "$CASE/err")"

new_case styled-download-no-length
publish 2.0.0
export FAKE_NO_LENGTH=1
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$INSTALL_SH" --version 2.0.0 >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
PLAIN=$(sed $'s/\033\\[[0-9;]*m//g' "$CASE/err")

test_case "styled, with no length announced the download is a counter, not a bar"
assert_equals "0" "$STATUS"
assert_contains "$PLAIN" "Downloading hm_linux_amd64…  $ASSET_SIZE B"
assert_not_contains "$PLAIN" "░"
assert_not_contains "$PLAIN" "%"
assert_contains "$(cat "$CASE/out")" "Downloaded hm 2.0.0 for linux/amd64, checksum verified"

new_case styled-download-failed
publish 2.0.0
rm "$FAKE_RELEASES/2.0.0/hm_linux_amd64"
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$INSTALL_SH" --version 2.0.0 >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
PLAIN=$(sed $'s/\033\\[[0-9;]*m//g' "$CASE/err")

test_case "styled, a failed download draws the download as failed, then the reason"
assert_equals "1" "$STATUS"
failed_line=$(printf '%s\n' "$PLAIN" | grep -n '✗ Downloading hm_linux_amd64' | head -n 1 | cut -d: -f1)
reason_line=$(printf '%s\n' "$PLAIN" | grep -n 'Could not download hm_linux_amd64' | head -n 1 | cut -d: -f1)
assert_equals "yes" "$([ -n "$failed_line" ] && [ -n "$reason_line" ] && [ "$failed_line" -lt "$reason_line" ] && echo yes || echo no)"
assert_equals "yes" "$(err_settles "$CASE/err")"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"
assert_equals "1" "$(printf '%s\n' "$PLAIN" | grep -c '✗ Downloading hm_linux_amd64')"

# fetch-alone.sh <install.sh> <fetch arguments...>: install.sh's fetch on its own, called the way a
# caller that goes on after a failure would — everything but main (the last line) sourced. Its
# stdout is the watcher's PID left behind, if any; then a line on stderr, and a pause in which a
# watcher left running would draw over it (killed after, so no case leaves one turning).
cat >"$LAB/fetch-alone.sh" <<'EOF'
#!/usr/bin/env bash
functions="$(mktemp -d "${TMPDIR:-/tmp}/hm-fetch.XXXXXX")"
# A file, not <(...): bash 3.2 sources nothing from a process substitution
sed '$d' "$1" >"$functions/install.sh"
source "$functions/install.sh"
shift
WORK="$functions/work"
mkdir "$WORK"
type fetch >/dev/null || exit 1
fetch "$@" || true
printf 'left=%s\n' "$SPINNER_PID"
printf 'more output\n' >&2
sleep 0.4
[ -z "$SPINNER_PID" ] || kill "$SPINNER_PID" 2>/dev/null
rm -rf "$functions"
EOF

new_case styled-fetch-failed-then-more
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$LAB/fetch-alone.sh" "$INSTALL_SH" \
    "https://github.com/hiberus-magento/hiberus-dockergento/releases/download/2.0.0/hm_linux_amd64" "$CASE/hm" progress \
    >"$CASE/out" 2>"$CASE/err" </dev/null)

test_case "styled, a failed download stops its own bar, drawn failed, so what is written next is never drawn over"
assert_equals "left=" "$(cat "$CASE/out")"
assert_contains "$(sed $'s/\033\\[[0-9;]*m//g' "$CASE/err")" "✗ Downloading hm_linux_amd64"
assert_equals "more output" "$(tail -c 12 "$CASE/err")"

# GITHUB_TOKEN reaches the API however fetch is asked to fetch it, its download watched or not:
# today only the binary is watched, and it is no API URL, but that is no rule fetch can count on
API_URL="https://api.github.com/repos/hiberus-magento/hiberus-dockergento/releases?per_page=100"
ASSET_URL="https://github.com/hiberus-magento/hiberus-dockergento/releases/download/2.0.0/hm_linux_amd64"

new_case styled-fetch-api-watched-wget
export GITHUB_TOKEN=token-for-the-test
(PATH="$WGET_TOOLS:$NO_CURL" CLICOLOR_FORCE=1 bash "$LAB/fetch-alone.sh" "$INSTALL_SH" "$API_URL" "$CASE/answer" progress \
    >"$CASE/out" 2>"$CASE/err" </dev/null)

test_case "styled, with wget only, a watched fetch of the API still carries GITHUB_TOKEN, never in wget's arguments"
assert_equals "$API_URL Authorization: Bearer token-for-the-test" "$(cat "$FAKE_CURL_LOG")"
assert_contains "$(cat "$FAKE_CURL_ARGV")" "-S"
assert_not_contains "$(cat "$FAKE_CURL_ARGV")" "token-for-the-test"
assert_equals "$(cat "$FAKE_API")" "$(cat "$CASE/answer")"

new_case styled-fetch-asset-watched-wget
publish 2.0.0
export GITHUB_TOKEN=token-for-the-test
(PATH="$WGET_TOOLS:$NO_CURL" CLICOLOR_FORCE=1 bash "$LAB/fetch-alone.sh" "$INSTALL_SH" "$ASSET_URL" "$CASE/hm" progress \
    >"$CASE/out" 2>"$CASE/err" </dev/null)

test_case "and a watched download that is not the API carries no token"
assert_equals "$ASSET_URL" "$(cat "$FAKE_CURL_LOG")"
assert_equals "$(cat "$FAKE_RELEASES/2.0.0/hm_linux_amd64")" "$(cat "$CASE/hm")"

new_case styled-fetch-api-watched-curl
export GITHUB_TOKEN=token-for-the-test
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$LAB/fetch-alone.sh" "$INSTALL_SH" "$API_URL" "$CASE/answer" progress \
    >"$CASE/out" 2>"$CASE/err" </dev/null)

test_case "styled, with curl, a watched fetch of the API carries GITHUB_TOKEN too, from a private file"
assert_equals "$API_URL Authorization: Bearer token-for-the-test" "$(cat "$FAKE_CURL_LOG")"
assert_equals "-rw-------" "$(cat "$FAKE_CURL_MODES")"
assert_not_contains "$(cat "$FAKE_CURL_ARGV")" "token-for-the-test"

# Interrupted mid-download: the bar's watcher is stopped by the same EXIT trap as the spinner
new_case styled-download-interrupted
publish 2.0.0
PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 FAKE_DOWNLOAD_DELAY=3 bash "$INSTALL_SH" --version 2.0.0 >"$CASE/out" 2>"$CASE/err" </dev/null &
installer=$!
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    grep -q '░' "$CASE/err" 2>/dev/null && break
    sleep 0.1
done
kill -TERM "$installer"
wait "$installer" 2>/dev/null
STATUS=$?

test_case "styled, an interrupted download stops the bar and leaves nothing behind"
assert_equals "yes" "$([ "$STATUS" -ne 0 ] && echo yes || echo no)"
assert_contains "$(cat "$CASE/err")" "░"
assert_equals "yes" "$(err_settles "$CASE/err")"
assert_equals "" "$(ls -A "$TMPDIR")"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"

new_case styled-pre
publish 2.0.0
publish 2.1.0-rc.1
api_releases 2.1.0-rc.1 2.0.0
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$INSTALL_SH" --pre >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
PLAIN=$(cat "$CASE/out" "$CASE/err" | sed $'s/\033\\[[0-9;]*m//g')

test_case "styled, a pre-release found by --pre is named as one"
assert_equals "0" "$STATUS"
assert_contains "$PLAIN" "✓ Latest pre-release: 2.1.0-rc.1"

new_case styled-version
publish 2.0.0
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$INSTALL_SH" --version 2.0.0 >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?

test_case "styled, a version asked for is not looked up, so there is no spinner"
assert_equals "0" "$STATUS"
assert_not_contains "$(cat "$CASE/out" "$CASE/err")" "Looking up"
assert_not_contains "$(cat "$CASE/out" "$CASE/err")" "Latest"

new_case styled-rate-limited
publish 2.0.0
export FAKE_API_STATUS=403
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 bash "$INSTALL_SH" --pre >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
PLAIN=$(sed $'s/\033\\[[0-9;]*m//g' "$CASE/err")

test_case "styled, a failed lookup draws the spinner as failed, then the reason"
assert_equals "1" "$STATUS"
assert_contains "$PLAIN" "✗ Looking up the latest release"
assert_not_contains "$PLAIN" "✓ Latest"
failed_line=$(printf '%s\n' "$PLAIN" | grep -n '✗ Looking up' | head -n 1 | cut -d: -f1)
reason_line=$(printf '%s\n' "$PLAIN" | grep -n 'rate limit was reached' | head -n 1 | cut -d: -f1)
assert_equals "yes" "$([ -n "$failed_line" ] && [ -n "$reason_line" ] && [ "$failed_line" -lt "$reason_line" ] && echo yes || echo no)"
assert_equals "yes" "$(err_settles "$CASE/err")"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"

# Interrupted while the spinner turns. A background job of a script ignores SIGINT, so it gets the
# SIGTERM a closing terminal sends; Ctrl-C reaches the same EXIT trap.
new_case styled-interrupted
publish 2.0.0
api_releases 2.0.0
PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 FAKE_API_DELAY=3 bash "$INSTALL_SH" --pre >"$CASE/out" 2>"$CASE/err" </dev/null &
installer=$!
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    grep -q 'Looking up' "$CASE/err" 2>/dev/null && break
    sleep 0.1
done
kill -TERM "$installer"
# Its stderr carries only bash's own "Terminated" notice about the job
wait "$installer" 2>/dev/null
STATUS=$?

test_case "styled, an interrupted lookup stops the spinner and leaves nothing behind"
assert_equals "yes" "$([ "$STATUS" -ne 0 ] && echo yes || echo no)"
assert_contains "$(cat "$CASE/err")" "Looking up the latest release…"
assert_equals "yes" "$(err_settles "$CASE/err")"
assert_equals "" "$(ls -A "$TMPDIR")"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"

new_case styled-no-color
publish 2.0.0
api_releases 2.0.0
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=1 NO_COLOR=1 bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
OUT=$(cat "$CASE/out" "$CASE/err")

test_case "NO_COLOR wins: the plain text, with no escape, tick or box"
assert_equals "0" "$STATUS"
assert_contains "$OUT" "hm 2.0.0 installed at $HM_SYSTEM_BIN/hm"
assert_not_contains "$OUT" $'\033['
assert_not_contains "$OUT" "✓"
assert_not_contains "$OUT" "╭"

new_case styled-forced-off
publish 2.0.0
api_releases 2.0.0
(PATH="$TOOLS:$BASE_PATH" CLICOLOR_FORCE=0 bash "$INSTALL_SH" >"$CASE/out" 2>"$CASE/err" </dev/null)
STATUS=$?
OUT=$(cat "$CASE/out" "$CASE/err")

test_case "CLICOLOR_FORCE=0 forces nothing: plain text without a terminal"
assert_equals "0" "$STATUS"
assert_not_contains "$OUT" "✓"

new_case uninstall-source-checkout
mkdir -p "$HOME/hm/bin"
printf '#!/bin/sh\necho "hm 2.1.0-dev"\n' >"$HOME/hm/bin/hm"
chmod +x "$HOME/hm/bin/hm"
git -C "$HOME/hm" init -q
ln -s "$HOME/hm/bin/hm" "$HM_SYSTEM_BIN/hm"
run_install --uninstall

test_case "--uninstall leaves the link to a contributor's build, and does not call it 1.x"
assert_equals "0" "$STATUS"
assert_equals "yes" "$([ -L "$HM_SYSTEM_BIN/hm" ] && echo yes || echo no)"
assert_contains "$OUT" "$HM_SYSTEM_BIN/hm is a link to a build of a source checkout and was left as it is."
assert_not_contains "$OUT" "1.x"

new_case source-checkout-spaces
publish 2.0.0
api_releases 2.0.0
mkdir -p "$HOME/my code/hm/bin"
printf '#!/bin/sh\necho "hm 2.1.0-dev"\n' >"$HOME/my code/hm/bin/hm"
chmod +x "$HOME/my code/hm/bin/hm"
git -C "$HOME/my code/hm" init -q
ln -s "$HOME/my code/hm/bin/hm" "$HM_SYSTEM_BIN/hm"
run_install

test_case "the command that brings a build back still works pasted when its path has spaces"
assert_equals "0" "$STATUS"
assert_contains "$OUT" "ln -sf $(printf '%q' "$HOME/my code/hm/bin/hm") $(printf '%q' "$HM_SYSTEM_BIN/hm")"

test_case "no case ever called jq"
assert_equals "" "$(cat "$LAB/jq.log" 2>/dev/null)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
