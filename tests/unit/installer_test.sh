#!/usr/bin/env bash
#
# install.sh installs hm 2 as one binary from a GitHub release: the latest of 2.0 or later, or the
# one asked for, verified against the release's checksums.txt. A 1.x link is replaced by the binary
# at the same path, its completion line by the new one, and its checkout is left where it is with a
# notice. installer.sh, the old name, only runs install.sh.
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
cat >"$TOOLS/curl" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_CURL_ARGV"
out="" url="" headers="" write=""
while [ $# -gt 0 ]; do
    case "$1" in
        -o) out="$2"; shift ;;
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
        if [ -n "${FAKE_API_STATUS:-}" ]; then status "$FAKE_API_STATUS"; exit 22; fi
        source="$FAKE_API" ;;
    https://github.com/*/releases/download/*) rest="${url#*/releases/download/}"; source="$FAKE_RELEASES/$rest" ;;
    https://raw.githubusercontent.com/*/install.sh) source="$HM_TEST_PROJECT_ROOT/install.sh" ;;
    *) status 404; exit 22 ;;
esac
[ -f "$source" ] || { status 404; exit 22; }
if [ -n "$out" ]; then cp "$source" "$out"; else cat "$source"; fi
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
    unset HM_VERSION HM_PRE HM_INSTALL_DIR FAKE_OS GITHUB_TOKEN FAKE_LATEST FAKE_API_STATUS
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

exists() { [ -e "$1" ] || [ -L "$1" ] && echo yes || echo no; }

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

test_case "a system directory that is not writable means ~/.local/bin, created, without sudo"
assert_equals "0" "$STATUS"
assert_equals "hm 2.0.0" "$("$HOME/.local/bin/hm" --version | head -n 1)"
assert_equals "" "$(cat "$FAKE_SUDO_LOG")"

test_case "with a warning when it is not on PATH"
assert_contains "$OUT" "$HOME/.local/bin is not in your PATH"
chmod u+w "$HM_SYSTEM_BIN"

new_case custom-dir
publish 2.0.0
api_releases 2.0.0
HM_INSTALL_DIR="$CASE/opt/bin" run_install

test_case "HM_INSTALL_DIR chooses the directory"
assert_equals "0" "$STATUS"
assert_equals "yes" "$(exists "$CASE/opt/bin/hm")"
assert_equals "no" "$(exists "$HM_SYSTEM_BIN/hm")"

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

test_case "the 1.x link is replaced by the binary at the same path"
assert_equals "0" "$STATUS"
assert_equals "no" "$([ -L "$HM_SYSTEM_BIN/hm" ] && echo yes || echo no)"
assert_equals "hm 2.0.0" "$("$HM_SYSTEM_BIN/hm" --version | head -n 1)"
assert_equals "no" "$(exists "$HOME/.local/bin/hm")"

test_case "with sudo, only for that directory, because it was not writable"
assert_contains "$(cat "$FAKE_SUDO_LOG")" "mv -f $HM_SYSTEM_BIN/.hm.install"
assert_not_contains "$(cat "$FAKE_SUDO_LOG")" "$HOME/"

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

test_case "no case ever called jq"
assert_equals "" "$(cat "$LAB/jq.log" 2>/dev/null)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
