#!/usr/bin/env bash
#
# Installer of hm 2 (Hiberus Dockergento): one self-contained binary from a GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- --version 2.0.0     # a particular release
#   curl -fsSL .../install.sh | bash -s -- --uninstall
#
# Options (each with an environment variable that does the same):
#   --version X   HM_VERSION=X       the release to install (2.0 or later); default the latest
#   --pre         HM_PRE=1           the latest may be a pre-release (X.Y.Z-rc.N)
#   --dir D       HM_INSTALL_DIR=D   the directory for the binary
#   --uninstall                      remove what this installer installed
#   -h, --help
#
# Written to be piped into bash: it never looks at $0 or at the directory it runs from, never reads
# standard input, and does nothing until the whole file has been read (main runs on the last line,
# so a download cut halfway runs nothing).
#
set -euo pipefail

REPOSITORY="hiberus-magento/hiberus-dockergento"
DOWNLOADS="https://github.com/$REPOSITORY/releases/download"
RELEASES_API="https://api.github.com/repos/$REPOSITORY/releases?per_page=100"

# Where a 1.x installation linked hm, and the default directory (holding only a link to the binary
# when it is not writable). Overridable so the tests can stand in for it without root.
SYSTEM_BIN="${HM_SYSTEM_BIN:-/usr/local/bin}"

# The installer's own files: the shell completion and a receipt of what it installed
STATE_DIR="$HOME/.hm"
COMPLETION_DIR="$STATE_DIR/completion"
RECEIPT="$STATE_DIR/install-receipt"

# Where the binary lives when the directory hm is run from is not the user's: that directory then
# holds only a link to it, made once with sudo, so that updating hm never needs root again
HOME_BIN="$STATE_DIR/bin"
HOME_BINARY="$HOME_BIN/hm"

# Marks the one profile line this installer owns, to replace or remove it and never add a second
PROFILE_MARK="# hm shell completion"

# Colours, ticks and a closing box in a terminal, or when CLICOLOR_FORCE asks for them; plain text
# otherwise, and always under NO_COLOR (https://no-color.org)
if [ -z "${NO_COLOR:-}" ] && { [ -t 1 ] || { [ -n "${CLICOLOR_FORCE:-}" ] && [ "${CLICOLOR_FORCE}" != "0" ]; }; }; then
    FANCY=1
    bold=$'\033[1m' dim=$'\033[2m' green=$'\033[0;32m' cyan=$'\033[0;36m' orange=$'\033[0;33m' red=$'\033[0;31m' reset=$'\033[0m'
else
    FANCY=""
    bold="" dim="" green="" cyan="" orange="" red="" reset=""
fi

say() { printf '%s\n' "$*"; }

# step <text>: one thing done — a green tick in a terminal, a dash otherwise
step() {
    local text="$*" first
    if [ -n "$FANCY" ]; then
        # Capitalised, by tr: bash 3.2, macOS's, has no ${text^}
        first="$(printf '%s' "${text:0:1}" | tr '[:lower:]' '[:upper:]')"
        printf '  %s✓%s %s%s\n' "$green" "$reset" "$first" "${text:1}"
    else
        printf '  - %s\n' "$text"
    fi
}

# header: the installer's name, in a terminal only
header() {
    [ -n "$FANCY" ] || return 0
    printf '\n  %s%shiberus%s %sDockergento%s %s· installer%s\n\n' "$bold" "$cyan" "$reset" "$bold" "$reset" "$dim" "$reset"
}

# box <first line> [lines...]: the lines in a rounded box, the first in bold, in a terminal only
box() {
    [ -n "$FANCY" ] || return 0
    local line width=0 length
    for line in "$@"; do
        length="$(printf '%s' "$line" | wc -m | tr -d ' ')"
        [ "$length" -gt "$width" ] && width="$length"
    done
    local rule
    rule="$(printf '%*s' "$((width + 4))" '' | sed 's/ /─/g')"
    printf '\n  %s╭%s╮%s\n' "$green" "$rule" "$reset"
    local first=true style
    for line in "$@"; do
        length="$(printf '%s' "$line" | wc -m | tr -d ' ')"
        style=""
        $first && style="$bold"
        first=false
        printf '  %s│%s  %s%s%s%*s  %s│%s\n' "$green" "$reset" "$style" "$line" "$reset" "$((width - length))" '' "$green" "$reset"
    done
    printf '  %s╰%s╯%s\n' "$green" "$rule" "$reset"
}

# spinner_start <text>: a spinner and the text on stderr while something runs, in a terminal only.
# The first frame is drawn here, so the text is on screen however quickly the work ends.
SPINNER_PID="" SPINNER_TEXT=""
spinner_start() {
    [ -n "$FANCY" ] || return 0
    SPINNER_TEXT="$1"
    printf '\r  %s⠋%s %s…' "$cyan" "$reset" "$SPINNER_TEXT" >&2
    # Exits on TERM only between frames, so it never leaves a sleep behind or draws after the
    # line is cleared. It ignores SIGINT, as background jobs of a script do: Ctrl-C ends the
    # installer, whose EXIT trap stops it.
    (
        trap 'exit 0' TERM
        frames=(⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏)
        i=1
        while :; do
            sleep 0.1 2>/dev/null || sleep 1
            printf '\r  %s%s%s %s…' "$cyan" "${frames[i]}" "$reset" "$SPINNER_TEXT" >&2
            i=$(((i + 1) % ${#frames[@]}))
        done
    ) </dev/null >/dev/null &
    SPINNER_PID=$!
}

# spinner_stop [failed]: the spinner stopped and its line cleared, or redrawn with a red cross
spinner_stop() {
    [ -n "$SPINNER_PID" ] || return 0
    kill "$SPINNER_PID" 2>/dev/null || true
    wait "$SPINNER_PID" 2>/dev/null || true
    SPINNER_PID=""
    printf '\r\033[K' >&2
    [ -z "${1:-}" ] || printf '  %s✗%s %s\n' "$red" "$reset" "$SPINNER_TEXT" >&2
}

# human_size <bytes>: in B, KB or MB, whole units
human_size() {
    if [ "$1" -ge 1048576 ]; then
        printf '%d MB' $(($1 / 1048576))
    elif [ "$1" -ge 1024 ]; then
        printf '%d KB' $(($1 / 1024))
    else
        printf '%d B' "$1"
    fi
}

# size_of <file>: its size in bytes so far, 0 while it is not there
size_of() {
    if [ -f "$1" ]; then wc -c <"$1" | tr -d ' '; else echo 0; fi
}

# announced_length <headers>: the Content-Length of the last response in a dump of headers — the
# file's, after any redirect — or nothing while none is known
announced_length() {
    [ -f "$1" ] || return 0
    tr -d '\r' <"$1" | sed -n 's/^[[:space:]]*[Cc][Oo][Nn][Tt][Ee][Nn][Tt]-[Ll][Ee][Nn][Gg][Tt][Hh]:[[:space:]]*\([0-9][0-9]*\).*/\1/p' | tail -n 1
}

# draw_download <bytes> <total> <frame>: one frame of the download on stderr — a bar against the
# total when it is known, a spinner and a counter when it is not
draw_download() {
    local got="$1" total="$2" width=24 percent filled full="" empty="" i=0
    if [ -n "$total" ] && [ "$total" -gt 0 ]; then
        percent=$((got * 100 / total))
        [ "$percent" -le 100 ] || percent=100
        filled=$((percent * width / 100))
        while [ "$i" -lt "$width" ]; do
            # Braced: bash 3.2 reads the first byte of █ as part of the variable's name
            if [ "$i" -lt "$filled" ]; then full="${full}█"; else empty="${empty}░"; fi
            i=$((i + 1))
        done
        printf '\r  %s%s%s%s%s  %3d%%  %s / %s\033[K' "$cyan" "$full" "$dim" "$empty" "$reset" "$percent" \
            "$(human_size "$got")" "$(human_size "$total")" >&2
    else
        printf '\r  %s%s%s %s…  %s\033[K' "$cyan" "$3" "$reset" "$SPINNER_TEXT" "$(human_size "$got")" >&2
    fi
}

# download_watch <file> <headers> <name>: the download of a file drawn on stderr while curl or wget
# write it silently, in a terminal only — its size polled against the length the server announced
# in the headers they dump. The spinner's discipline: it runs as SPINNER_PID, so the EXIT trap stops
# it if the installer is interrupted, and fetch stops it once the download ends: drawn whole when it
# finished, as "✗ Downloading <name>" when it failed.
download_watch() {
    [ -n "$FANCY" ] || return 0
    SPINNER_TEXT="Downloading $3"
    draw_download 0 "" ⠋
    (
        trap 'exit 0' TERM
        frames=(⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏)
        i=1
        while :; do
            sleep 0.1 2>/dev/null || sleep 1
            draw_download "$(size_of "$1")" "$(announced_length "$2")" "${frames[i]}"
            i=$(((i + 1) % ${#frames[@]}))
        done
    ) </dev/null >/dev/null &
    SPINNER_PID=$!
}

# download_done <file> <headers>: the watch stopped, its last frame drawn whole whatever the poll
# last saw, then cleared as the spinner leaves its line: the tick that follows says it is done
download_done() {
    [ -n "$SPINNER_PID" ] || return 0
    kill "$SPINNER_PID" 2>/dev/null || true
    wait "$SPINNER_PID" 2>/dev/null || true
    SPINNER_PID=""
    draw_download "$(size_of "$1")" "$(announced_length "$2")" ⠋
    printf '\r\033[K' >&2
}

# warn stops a running spinner first, as die does: a line written while it turns would be drawn
# onto its half-written frame and then overwritten by the next one
warn() {
    spinner_stop
    printf '%s%s%s%s\n' "${FANCY:+  ! }" "$orange" "$*" "$reset" >&2
}
die() {
    spinner_stop failed
    printf '%s%s%s\n' "$red" "$1" "$reset" >&2
    exit "${2:-1}"
}

usage() {
    cat <<'EOF'
Installs hm 2 (Hiberus Dockergento) from a GitHub release.

  curl -fsSL https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh | bash
  curl -fsSL .../install.sh | bash -s -- [options]

Options (each with an environment variable that does the same):
  --version X   HM_VERSION=X       the release to install (2.0 or later); default the latest
  --pre         HM_PRE=1           the latest may be a pre-release (X.Y.Z-rc.N)
  --dir D       HM_INSTALL_DIR=D   the directory for the binary
  --uninstall                      remove what this installer installed
  -h, --help
EOF
}

# ------------------------------------------------------------------------------------------ versions

# A release tag: X.Y.Z with an optional pre-release part, no "v"
is_version() {
    [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]
}

major_of() { printf '%s' "${1%%.*}"; }

refuse_shell_version() {
    die "'$1' is a version of the shell tool (1.x); this installer installs hm 2.0 and later only. Nothing was installed.
1.x is installed from git, in a directory of its own: git clone -b $1 https://github.com/$REPOSITORY.git ~/hm-1.x, then run ~/hm-1.x/bin/run" 2
}

# version_gt A B: whether A is a later version than B. A final release is later than its
# pre-releases; pre-release parts compare field by field, numerically when both are numbers.
version_gt() {
    local a="$1" b="$2" a_core a_pre b_core b_pre i x y
    a_core="${a%%-*}" b_core="${b%%-*}"
    a_pre="" b_pre=""
    [[ "$a" == *-* ]] && a_pre="${a#*-}"
    [[ "$b" == *-* ]] && b_pre="${b#*-}"

    local -a ac bc
    IFS=. read -r -a ac <<<"$a_core"
    IFS=. read -r -a bc <<<"$b_core"
    for i in 0 1 2; do
        x="${ac[$i]:-0}" y="${bc[$i]:-0}"
        if ((10#$x > 10#$y)); then return 0; fi
        if ((10#$x < 10#$y)); then return 1; fi
    done

    [ "$a_pre" == "$b_pre" ] && return 1
    [ -z "$a_pre" ] && return 0
    [ -z "$b_pre" ] && return 1

    local -a ap bp
    IFS=. read -r -a ap <<<"$a_pre"
    IFS=. read -r -a bp <<<"$b_pre"
    for ((i = 0; i < ${#ap[@]} || i < ${#bp[@]}; i++)); do
        x="${ap[$i]:-}" y="${bp[$i]:-}"
        [ "$x" == "$y" ] && continue
        [ -z "$x" ] && return 1
        [ -z "$y" ] && return 0
        if [[ "$x" =~ ^[0-9]+$ && "$y" =~ ^[0-9]+$ ]]; then
            ((10#$x > 10#$y)) && return 0 || return 1
        fi
        [[ "$x" > "$y" ]] && return 0 || return 1
    done
    return 1
}

# --------------------------------------------------------------------------------------- downloading

# token_header: a file holding the Authorization header for GITHUB_TOKEN, readable by this user
# only, so the token never appears in curl's or wget's arguments (which ps shows to everyone)
token_header() {
    local file="$WORK/github-auth"
    (umask 077 && printf 'Authorization: Bearer %s\n' "$GITHUB_TOKEN" >"$file") || return 1
    printf '%s' "$file"
}

# fetch <url> <file> [progress]: with curl, or wget when there is no curl. GITHUB_TOKEN goes to the
# API only, through a 0600 file removed afterwards. FETCH_STATUS is the HTTP status when curl could
# tell it. With progress, and in a terminal, hm's own download bar on stderr (download_watch), the
# downloader silent.
fetch() {
    local url="$1" out="$2" header="" status=0
    local -a auth=() quiet=(-fsSL) dumping=()
    local dump=""
    FETCH_STATUS=""

    if [ -n "${3:-}" ] && [ -n "$FANCY" ]; then
        # The headers are dumped as they arrive, so the watch knows the length before the body
        dump="$out.headers"
        : >"$dump"
        dumping=(-D "$dump")
        download_watch "$out" "$dump" "${url##*/}"
    fi

    if [ -n "${GITHUB_TOKEN:-}" ] && [[ "$url" == https://api.github.com/* ]]; then
        header="$(token_header)" || die "Could not write a private file for GITHUB_TOKEN in $WORK."
        auth=(-H "@$header")
    fi

    if command -v curl >/dev/null 2>&1; then
        FETCH_STATUS="$(curl "${quiet[@]}" --retry 2 ${auth[@]+"${auth[@]}"} ${dumping[@]+"${dumping[@]}"} -o "$out" -w '%{http_code}' "$url")" || status=$?
    elif command -v wget >/dev/null 2>&1; then
        quiet=(-q)
        if [ -n "$dump" ]; then
            # -S writes the server's answer, headers and all, with wget's own log: both into the
            # dump the watch reads, and nothing onto the terminal. Never the API, so no header here
            wget -S -O "$out" "$url" 2>"$dump" || status=$?
        elif [ -n "$header" ]; then
            # wget reads the header from a wgetrc of its own rather than from its arguments
            (umask 077 && printf 'header = %s\n' "$(cat "$header")" >"$header.wgetrc") &&
                wget "${quiet[@]}" --config="$header.wgetrc" -O "$out" "$url" || status=$?
        else
            wget "${quiet[@]}" -O "$out" "$url" || status=$?
        fi
    else
        die "Neither curl nor wget is installed; install one of them and run this again."
    fi

    [ -z "$header" ] || rm -f "$header" "$header.wgetrc"

    # A finished download is drawn whole and cleared; a failed one is stopped here and drawn
    # failed, so no caller, whether it dies, warns or goes on, can leave it drawing over its output
    if [ -n "$dump" ]; then
        if [ "$status" -eq 0 ]; then
            download_done "$out" "$dump"
        else
            spinner_stop failed
        fi
        rm -f "$dump"
    fi

    return "$status"
}

# latest_redirect: the tag GitHub's releases/latest page redirects to, without the API and its
# rate limit; nothing when it cannot tell
latest_redirect() {
    local url="https://github.com/$REPOSITORY/releases/latest" location="" headers=""

    if command -v curl >/dev/null 2>&1; then
        location="$(curl -fsS -o /dev/null -w '%{redirect_url}' "$url" 2>/dev/null)" || location=""
    elif command -v wget >/dev/null 2>&1; then
        # wget exits non-zero when told to follow no redirect and given one, which is the answer
        # here: its status is ignored and the Location header read from what it printed
        headers="$(wget -q -S --max-redirect=0 -O /dev/null "$url" 2>&1)" || true
        location="$(printf '%s\n' "$headers" | awk 'tolower($1) == "location:" { print $2; exit }' | tr -d '\r')"
    fi

    case "$location" in
        */releases/tag/*) printf '%s' "${location##*/releases/tag/}" ;;
    esac
}

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{ print $1 }'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{ print $1 }'
    else
        die "Neither sha256sum nor shasum is installed, so the download cannot be verified. Nothing was installed."
    fi
}

# The release tags of the API answer that are not drafts, one per line. Read without jq: the tag
# comes before "draft" in every release object, and text inside a release's notes has its quotes
# escaped, so it never matches.
release_tags() {
    tr -d '\n\r' <"$1" |
        grep -o '"tag_name": *"[^"]*"\|"draft": *[a-z]*' |
        awk -F'"' '/tag_name/ { tag = $4; next } /draft/ { if ($0 !~ /true/ && tag != "") print tag; tag = "" }'
}

# The newest release of 2.0 or later; a pre-release only when they are allowed. The latest final
# release is read from the releases/latest redirect, which has no rate limit; the API is asked only
# for pre-releases, or when that redirect does not name a release of 2.0 or later.
latest_version() {
    local answer="$WORK/releases.json" tag best="" best_pre=""

    if [ "$PRE" != "1" ]; then
        tag="$(latest_redirect)"
        tag="${tag#v}"
        if is_version "$tag" && [[ "$tag" != *-* ]] && [ "$(major_of "$tag")" -ge 2 ]; then
            printf '%s' "$tag"
            return 0
        fi
    fi

    if ! fetch "$RELEASES_API" "$answer"; then
        case "$FETCH_STATUS" in
            403 | 429) die "GitHub's API rate limit was reached (HTTP $FETCH_STATUS) asking for the releases of $REPOSITORY. Wait an hour, set GITHUB_TOKEN to raise the limit, or name a version to skip the lookup: --version X.Y.Z" ;;
        esac
        die "Could not ask GitHub for the releases of $REPOSITORY (offline, or the API rate limit; GITHUB_TOKEN raises it). Name a version to skip the lookup: --version X.Y.Z"
    fi

    while IFS= read -r tag; do
        tag="${tag#v}"
        is_version "$tag" || continue
        [ "$(major_of "$tag")" -ge 2 ] || continue
        if [[ "$tag" == *-* ]]; then
            if [ -z "$best_pre" ] || version_gt "$tag" "$best_pre"; then best_pre="$tag"; fi
            [ "$PRE" == "1" ] || continue
        fi
        if [ -z "$best" ] || version_gt "$tag" "$best"; then best="$tag"; fi
    done < <(release_tags "$answer")

    if [ -z "$best" ]; then
        if [ -n "$best_pre" ]; then
            die "No final release of hm 2.0 or later has been published yet; the latest pre-release is $best_pre. Install it with --pre (or HM_PRE=1), or name it: --version $best_pre"
        fi
        die "No release of hm 2.0 or later has been published yet at https://github.com/$REPOSITORY/releases. Nothing was installed."
    fi

    printf '%s' "$best"
}

platform_asset() {
    local system architecture

    case "$(uname -s)" in
        Darwin) system="darwin" ;;
        Linux) system="linux" ;;
        *) die "hm runs on macOS and Linux; this is $(uname -s)." ;;
    esac

    case "$(uname -m)" in
        arm64 | aarch64) architecture="arm64" ;;
        x86_64 | amd64) architecture="amd64" ;;
        *) die "There is no hm binary for the $(uname -m) architecture (only amd64 and arm64)." ;;
    esac

    printf 'hm_%s_%s' "$system" "$architecture"
}

# download <version>: the platform binary of that release into $WORK/hm, verified against the
# release's checksums.txt. Fails closed: no checksums, no line for the asset, or another digest.
download() {
    local version="$1" asset expected actual
    asset="$(platform_asset)"

    # The binary is the one download long enough to watch; the checksums stay silent
    fetch "$DOWNLOADS/$version/$asset" "$WORK/hm" progress ||
        die "Could not download $asset of release $version ($DOWNLOADS/$version/$asset). Nothing was installed."
    fetch "$DOWNLOADS/$version/checksums.txt" "$WORK/checksums.txt" ||
        die "Could not download the checksums of release $version. Nothing was installed."

    expected="$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1 }' "$WORK/checksums.txt")"
    actual="$(sha256_of "$WORK/hm")"

    if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
        rm -f "$WORK/hm"
        die "The downloaded $asset does not match the checksum published with release $version; it was discarded. Nothing was installed."
    fi

    chmod 755 "$WORK/hm"

    "$WORK/hm" --version >/dev/null 2>&1 ||
        die "The downloaded $asset does not run on this machine. Nothing was installed."
}

# ------------------------------------------------------------------------------ what is installed

# version_of <binary>: the version an hm binary reports, or nothing
version_of() {
    "$1" --version 2>/dev/null | head -n 1 | sed $'s/\033\\[[0-9;]*m//g' |
        grep -o '[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[-0-9A-Za-z.]*' | head -n 1 || true
}

receipt_field() {
    [ -f "$RECEIPT" ] || return 0
    sed -n "s/^$1=//p" "$RECEIPT" | head -n 1
}

# is_hm2 <path>: a binary (not a link) that is hm 2 — the one in the receipt, or one reporting 2.x
is_hm2() {
    local path="$1" version
    [ -f "$path" ] && [ ! -L "$path" ] && [ -x "$path" ] || return 1

    if [ "$(receipt_field path)" == "$path" ] && [ "$(receipt_field sha256)" == "$(sha256_of "$path")" ]; then
        return 0
    fi

    version="$(version_of "$path")"
    [ -n "$version" ] && [ "$(major_of "$version")" -ge 2 ] && "$path" --version 2>/dev/null | head -n 1 | grep -q 'hm'
}

# is_legacy <link> <checkout>: whether the checkout a link points into is a 1.x installation: the
# link names bin/run, 1.x's entry point, or the checkout still has bin/run or console/. A 2.x
# source checkout — a contributor's clone, linked to the bin/hm built in it — is not one.
is_legacy() {
    [[ "$(readlink "$1")" == */bin/run ]] || [ -e "$2/bin/run" ] || [ -d "$2/console" ]
}

# checkout_of <link>: the git checkout a link points into, <checkout>/bin/run or <checkout>/bin/hm:
# a 1.x installation or a contributor's build (is_legacy tells them apart). Nothing otherwise.
checkout_of() {
    local link="$1" target checkout
    [ -L "$link" ] || return 0
    target="$(readlink "$link")"
    case "$target" in
        # ~/.hm/bin/hm ends in bin/hm too, and is no clone even when it is missing
        */.hm/bin/hm) return 0 ;;
        */bin/run | */bin/hm) ;;
        *) return 0 ;;
    esac
    checkout="${target%/bin/*}"
    # bin/run is 1.x's entry point and nothing else's; a link to a bin/hm counts only inside a clone
    if [[ "$target" == */bin/run ]] || [ -e "$checkout/.git" ] || [ ! -e "$target" ]; then
        printf '%s' "$checkout"
    fi
}

# is_layout_link <path>: a link to ~/.hm/bin/hm, made by this installer or by hm update
is_layout_link() {
    [ -L "$1" ] && [ "$(readlink "$1")" == "$HOME_BINARY" ]
}

# ------------------------------------------------------------------------------ writing files

# as_needed <dir> <command...>: as root only when the directory is not writable
as_needed() {
    local dir="$1"
    shift
    if [ -w "$dir" ]; then
        "$@"
    elif command -v sudo >/dev/null 2>&1; then
        if [ -z "${SUDO_ANNOUNCED:-}" ]; then
            say "${FANCY:+  ${dim}›${reset} }Writing to $dir needs root; sudo may ask for your password."
            SUDO_ANNOUNCED=1
        fi
        sudo "$@"
    else
        die "$dir is not writable and sudo is not available. Nothing was changed. Choose a directory of your own: --dir ~/.local/bin"
    fi
}

# place <source> <target>: the binary at target, replaced in one rename so there is never a half
# written hm, and a link at target is replaced by the binary itself
place() {
    local source="$1" target="$2" dir partial
    dir="$(dirname "$target")"
    partial="$dir/.hm.install.$$"

    if ! { as_needed "$dir" cp "$source" "$partial" &&
        as_needed "$dir" chmod 755 "$partial" &&
        as_needed "$dir" mv -f "$partial" "$target"; }; then
        as_needed "$dir" rm -f "$partial" 2>/dev/null || true
        die "Could not write $target. Nothing was changed."
    fi
}

# place_link <link>: link made a link to ~/.hm/bin/hm in one rename, so whatever was there (a 1.x
# link, an earlier binary) runs until the new link replaces it; fails, changing nothing, when even
# sudo cannot write its directory
place_link() {
    local link="$1" dir partial
    dir="$(dirname "$link")"
    partial="$dir/.hm.install.$$"

    if as_needed "$dir" ln -s "$HOME_BINARY" "$partial" && as_needed "$dir" mv -f "$partial" "$link"; then
        return 0
    fi
    as_needed "$dir" rm -f "$partial" 2>/dev/null || true
    return 1
}

# place_layout <source>: the binary at ~/.hm/bin/hm and LINK a link to it, sudo for the link only.
# Fails when the link cannot be made — no sudo, or sudo refused — leaving LINK as it was and
# ~/.hm/bin/hm as it was: absent, or the hm that was there, put back.
place_layout() {
    local source="$1" previous=""
    if [ ! -w "$(dirname "$LINK")" ] && ! command -v sudo >/dev/null 2>&1; then return 1; fi

    # Copied with its mode into WORK, which goes away whatever happens, so no copy is left behind
    if [ -e "$HOME_BINARY" ] || [ -L "$HOME_BINARY" ]; then
        previous="$WORK/previous-hm"
        cp -p "$HOME_BINARY" "$previous" || die "Could not keep a copy of $HOME_BINARY. Nothing was changed."
    fi
    mkdir -p "$HOME_BIN"
    place "$source" "$HOME_BINARY"
    place_link "$LINK" && return 0

    if [ -z "$previous" ]; then
        rm -f "$HOME_BINARY"
        rmdir "$HOME_BIN" 2>/dev/null || true
    elif ! { cp -p "$previous" "$HOME_BIN/.hm.restore.$$" && mv -f "$HOME_BIN/.hm.restore.$$" "$HOME_BINARY"; }; then
        rm -f "$HOME_BIN/.hm.restore.$$"
        die "Linking $LINK needs root, sudo is not available or was refused, and the hm that was in $HOME_BINARY could not be put back: it now holds the downloaded one."
    fi
    return 1
}

# resolve_link <path>: the regular file a chain of links ends at; fails when it ends nowhere
resolve_link() {
    local path="$1" target hops=0
    while [ -L "$path" ]; do
        hops=$((hops + 1))
        [ "$hops" -le 40 ] || return 1
        target="$(readlink "$path")" || return 1
        case "$target" in
            /*) path="$target" ;;
            *) path="$(dirname "$path")/$target" ;;
        esac
    done
    [ -f "$path" ] || return 1
    printf '%s' "$path"
}

# same_owner_single_link <file>: whether a file put in its place by this user would be the same to
# everyone else: it has one link, and this user owns it (or is root, whose cp -p keeps the owner)
same_owner_single_link() {
    local links owner me
    read -r _ links owner _ < <(LC_ALL=C ls -ln "$1" 2>/dev/null) || return 1
    me="$(id -u)" || return 1
    [ "$links" == "1" ] && { [ "$owner" == "$me" ] || [ "$me" == "0" ]; }
}

# profile_by_hand <profile> <reason> <mark> [line]: what the user has to do, since it was not done
profile_by_hand() {
    warn "Could not edit $1 safely ($2); it was left exactly as it was."
    if [ -n "${4:-}" ]; then
        warn "Add this line to it yourself:"
        warn "  $4"
    else
        warn "Remove the line containing \"$3\" from it yourself."
    fi
}

# edit_profile <profile> <mark> [line]: the profile without the lines containing mark, and with
# line appended when given. Never truncates it: the new content is written to a temporary file in
# the same directory with the same permissions, checked, and renamed over the file in one step. A
# profile that is a link (managed dotfiles) is edited through the link, at the file it points to,
# and stays a link. A file owned by another user or with other hard links is not edited, since
# the rename would change its owner or split it from its links. The first edit keeps a copy of the
# original in <profile>.hm-backup. Whatever
# fails, the profile is as it was, the line to change by hand is printed, and it returns 1.
edit_profile() {
    local profile="$1" mark="$2" line="${3:-}" real dir temporary kept expected status

    if [ ! -e "$profile" ] && [ ! -L "$profile" ]; then
        [ -n "$line" ] || return 0
        if (umask 022 && printf '%s\n' "$line" >"$profile") 2>/dev/null; then return 0; fi
        profile_by_hand "$profile" "it could not be created" "$mark" "$line"
        return 1
    fi

    if ! real="$(resolve_link "$profile")"; then
        profile_by_hand "$profile" "it is a link to nothing, or not a regular file" "$mark" "$line"
        return 1
    fi
    if [ ! -r "$real" ] || [ ! -w "$real" ]; then
        profile_by_hand "$profile" "$real is not writable" "$mark" "$line"
        return 1
    fi
    # The rename puts a new file in its place: owned by whoever runs this, and no longer the file
    # its other hard links name. Either would change the profile behind the user's back.
    if ! same_owner_single_link "$real"; then
        profile_by_hand "$profile" "$real belongs to another user or has other hard links" "$mark" "$line"
        return 1
    fi

    dir="$(dirname "$real")"
    if ! temporary="$(mktemp "$dir/.$(basename "$real").hm-XXXXXX" 2>/dev/null)"; then
        profile_by_hand "$profile" "no temporary file could be created in $dir" "$mark" "$line"
        return 1
    fi

    status=0
    kept="$(grep -c -v -F -e "$mark" "$real")" || status=$?
    expected="$kept"
    [ -z "$line" ] || expected=$((kept + 1))

    # cp -p first so the new file has the profile's mode; then the content, then the check that it
    # is every kept line, in order, plus the one added
    if [ "$status" -le 1 ] &&
        cp -p "$real" "$temporary" 2>/dev/null &&
        { grep -v -F -e "$mark" <"$real" >"$temporary"; [ $? -le 1 ]; } &&
        { [ -z "$line" ] || printf '%s\n' "$line" >>"$temporary"; } &&
        [ "$(grep -c '' "$temporary")" == "$expected" ] &&
        { grep -v -F -e "$mark" "$real" || [ $? -eq 1 ]; } | cmp -s - <(awk -v n="$kept" 'NR <= n' "$temporary") &&
        { [ -e "$profile.hm-backup" ] || [ -L "$profile.hm-backup" ] || cp -p "$real" "$profile.hm-backup" 2>/dev/null; } &&
        mv -f "$temporary" "$real" 2>/dev/null; then
        return 0
    fi

    rm -f "$temporary"
    profile_by_hand "$profile" "writing or checking its new content failed" "$mark" "$line"
    return 1
}

# The profile the user's shell reads, the way hm 1.x chose it
profile_of() {
    case "$1" in
        zsh) printf '%s' "$HOME/.zshrc" ;;
        bash)
            if [ "$(uname -s)" == "Darwin" ]; then printf '%s' "$HOME/.bash_profile"; else printf '%s' "$HOME/.bashrc"; fi
            ;;
    esac
}

LEGACY_COMPLETION="console/hm-completion.bash"
PROFILES=("$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.bash_profile")

# Removes hm 1.x's completion line from every profile it may be in; prints those it changed
drop_legacy_completion() {
    local profile
    for profile in "${PROFILES[@]}"; do
        if [ -f "$profile" ] && grep -q -F "$LEGACY_COMPLETION" "$profile"; then
            if edit_profile "$profile" "$LEGACY_COMPLETION"; then
                CHANGES+=("removed the 1.x completion line from $profile")
            else
                CHANGES+=("the 1.x completion line is still in $profile: remove it by hand")
            fi
        fi
    done
}

# The completion written by the binary itself, sourced from one profile line
install_completion() {
    local binary="$1" shell profile file line
    shell="$(basename "${SHELL:-}")"
    profile="$(profile_of "$shell")"

    if [ -z "$profile" ]; then
        CHANGES+=("no shell completion: $shell is not zsh or bash (hm completion zsh|bash prints it)")
        return 0
    fi

    file="$COMPLETION_DIR/hm.$shell"
    mkdir -p "$COMPLETION_DIR"
    if ! "$binary" completion "$shell" >"$WORK/completion" 2>/dev/null || [ ! -s "$WORK/completion" ]; then
        warn "This hm does not print its $shell completion (hm completion $shell); the completion was not installed."
        return 0
    fi
    mv -f "$WORK/completion" "$file"

    # Unexpanded $HOME, and guarded: deleting ~/.hm must not break every new terminal
    # shellcheck disable=SC2016
    line="[ -r \"\$HOME/.hm/completion/hm.$shell\" ] && . \"\$HOME/.hm/completion/hm.$shell\" $PROFILE_MARK"

    if grep -q -x -F "$line" "$profile" 2>/dev/null && [ "$(grep -c -F "$PROFILE_MARK" "$profile")" == "1" ]; then
        CHANGES+=("shell completion: $file (already sourced from $profile)")
        return 0
    fi

    if edit_profile "$profile" "$PROFILE_MARK" "$line"; then
        CHANGES+=("shell completion: $file, sourced from $profile")
    else
        CHANGES+=("shell completion: $file, not yet sourced from $profile: add the line above by hand")
    fi
}

# The notice for a 1.x checkout that this installation no longer uses. It is left where it is.
legacy_notice() {
    local checkout="$1" changed
    [ -n "$checkout" ] && [ -d "$checkout" ] || return 0

    say ""
    say "${bold}The 1.x installation at $checkout is no longer used${reset}: hm is now the single binary above."
    if command -v git >/dev/null 2>&1 && git -C "$checkout" rev-parse --git-dir >/dev/null 2>&1; then
        changed="$(git -C "$checkout" status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
        if [ "$changed" != "0" ]; then
            say "${orange}It has local changes ($changed changed files in git status): keep anything you need from it before deleting it.${reset}"
        else
            say "It has no local changes."
        fi
    fi
    say "It was left in place. To delete it: rm -rf $checkout"
}

# copy_new <source> <target>: source's content at target, readable by this user only; fails, and
# changes nothing, when target exists
copy_new() {
    local temporary status=0
    temporary="$(mktemp "$(dirname "$2")/.$(basename "$2").hm-XXXXXX" 2>/dev/null)" || return 1
    cat "$1" >"$temporary" 2>/dev/null && ln "$temporary" "$2" 2>/dev/null || status=1
    rm -f "$temporary"
    return "$status"
}

# json_object <file>: whether the file is one valid JSON object (RFC 8259), the check hm itself
# makes before it copies these settings (encoding/json's Valid and a "{" first). Without jq: a
# recursive-descent parser in awk, byte by byte.
json_object() {
    LC_ALL=C awk '
        { text = text $0 "\n" }
        function fail() { bad = 1; return 0 }
        function ws() { while (pos <= len && index(" \t\r\n", substr(text, pos, 1))) pos++ }
        function ch() { return substr(text, pos, 1) }
        function str(   c, i) {
            if (ch() != "\"") return fail()
            pos++
            while (pos <= len) {
                c = ch()
                if (c == "\"") { pos++; return 1 }
                if (c < " ") return fail()
                if (c == "\\") {
                    pos++; c = ch()
                    if (c == "u") {
                        for (i = 1; i <= 4; i++) if (!index("0123456789abcdefABCDEF", substr(text, pos + i, 1)) || substr(text, pos + i, 1) == "") return fail()
                        pos += 4
                    } else if (c == "" || !index("\"\\/bfnrt", c)) return fail()
                }
                pos++
            }
            return fail()
        }
        function digits(   start) { start = pos; while (pos <= len && index("0123456789", ch())) pos++; return pos > start }
        function num() {
            if (ch() == "-") pos++
            if (ch() == "0") pos++
            else if (!digits()) return fail()
            if (ch() == ".") { pos++; if (!digits()) return fail() }
            if (ch() == "e" || ch() == "E") { pos++; if (ch() == "+" || ch() == "-") pos++; if (!digits()) return fail() }
            return 1
        }
        function word(w) { if (substr(text, pos, length(w)) != w) return fail(); pos += length(w); return 1 }
        function value(   c) {
            ws(); c = ch()
            if (c == "{") return obj()
            if (c == "[") return arr()
            if (c == "\"") return str()
            if (c == "t") return word("true")
            if (c == "f") return word("false")
            if (c == "n") return word("null")
            if (c == "-" || index("0123456789", c)) return num()
            return fail()
        }
        function obj() {
            pos++; ws()
            if (ch() == "}") { pos++; return 1 }
            while (1) {
                ws(); if (!str()) return 0
                ws(); if (ch() != ":") return fail()
                pos++; if (!value()) return 0
                ws()
                if (ch() == "}") { pos++; return 1 }
                if (ch() != ",") return fail()
                pos++
            }
        }
        function arr() {
            pos++; ws()
            if (ch() == "]") { pos++; return 1 }
            while (1) {
                if (!value()) return 0
                ws()
                if (ch() == "]") { pos++; return 1 }
                if (ch() != ",") return fail()
                pos++
            }
        }
        END {
            len = length(text); pos = 1; bad = 0
            ws()
            if (ch() != "{" || !value() || bad) exit 1
            ws()
            exit (pos <= len ? 1 : 0)
        }
    ' "$1"
}

# migrate_settings <checkout>: a 1.x installation kept the store and admin settings of `hm install`
# in <checkout>/data/config.json; hm 2 keeps them in ~/.hm/config.json. Copied once, when that is
# absent and the old file is a JSON object; never overwritten.
migrate_settings() {
    local source="$1/data/config.json" target="$STATE_DIR/config.json"
    # A link is not followed: it may point anywhere, and what is copied ends up in ~/.hm
    [ -n "$1" ] && [ -f "$source" ] && [ ! -L "$source" ] && [ ! -e "$target" ] && [ ! -L "$target" ] || return 0

    if ! json_object "$source"; then
        CHANGES+=("the 1.x install settings $source are not a JSON object and were not copied")
        return 0
    fi

    # 0600 (mktemp's mode), as it can hold the admin password; put in place with ln, which never
    # replaces whatever appeared there meanwhile, and only once it is whole
    if mkdir -p "$STATE_DIR" && copy_new "$source" "$target"; then
        CHANGES+=("copied the 1.x install settings $source to $target")
    else
        CHANGES+=("could not copy the 1.x install settings $source to $target: copy it by hand")
    fi
}

# write_receipt <binary> <version> [link]: link is where hm is run from when the binary is behind one
write_receipt() {
    mkdir -p "$STATE_DIR"
    {
        printf 'path=%s\nversion=%s\nsha256=%s\n' "$1" "$2" "$(sha256_of "$1")"
        [ -z "${3:-}" ] || printf 'link=%s\n' "$3"
    } >"$RECEIPT"
}

# ------------------------------------------------------------------------------------- installing

on_path() {
    case ":$PATH:" in *":$1:"*) return 0 ;; *) return 1 ;; esac
}

# writable_dir <dir>: whether this user can write dir, or create it when it does not exist yet
writable_dir() {
    local dir="$1"
    while [ ! -d "$dir" ] && [ "$dir" != "/" ]; do dir="$(dirname "$dir")"; done
    [ -w "$dir" ]
}

# Where hm goes, and what is there now. Sets COMMAND, the path hm is run from; TARGET, where the
# binary is written (COMMAND itself, or ~/.hm/bin/hm behind a link); LINK, COMMAND when it is such a
# link; MAKE_LINK when that link is still to be made; CURRENT_BINARY, the hm 2 binary in use; and
# LEGACY_CHECKOUT, SOURCE_CHECKOUT and REPLACING, what COMMAND is now.
choose_target() {
    local dir checkout receipt_path receipt_link

    TARGET="" COMMAND="" LINK="" MAKE_LINK="" CURRENT_BINARY="" LEGACY_CHECKOUT="" SOURCE_CHECKOUT="" REPLACING=""
    receipt_path="$(receipt_field path)"
    receipt_link="$(receipt_field link)"

    if [ -n "$INSTALL_DIR" ]; then
        dir="$INSTALL_DIR"
    elif [ -n "$receipt_link" ] && is_layout_link "$receipt_link" && is_hm2 "$HOME_BINARY"; then
        # The binary behind a link last time: the link is where hm is run from
        dir="$(dirname "$receipt_link")"
    elif [ -n "$receipt_path" ] && [ "$receipt_path" != "$HOME_BINARY" ] &&
        { is_hm2 "$receipt_path" || is_layout_link "$receipt_path"; }; then
        # Where this installer put hm last time, --dir included: running it again upgrades that one.
        # A link to ~/.hm/bin/hm there is hm update having moved the binary since: the receipt is
        # this installer's alone, so it still names the old path.
        dir="$(dirname "$receipt_path")"
    else
        # An hm already in the system directory — a 1.x link, an earlier hm 2 or a link to
        # ~/.hm/bin/hm — is replaced at the same path, so there is only ever one hm
        checkout="$(checkout_of "$SYSTEM_BIN/hm")"
        if [ -n "$checkout" ] || is_hm2 "$SYSTEM_BIN/hm" || is_layout_link "$SYSTEM_BIN/hm"; then
            dir="$SYSTEM_BIN"
        elif is_hm2 "$HOME/.local/bin/hm"; then
            dir="$HOME/.local/bin"
        elif [ -d "$SYSTEM_BIN" ] && [ ! -e "$SYSTEM_BIN/hm" ] && [ ! -L "$SYSTEM_BIN/hm" ]; then
            # Writable or not: when it is not, only the link goes here
            dir="$SYSTEM_BIN"
        else
            dir="$HOME/.local/bin"
        fi
    fi

    COMMAND="$dir/hm"
    TARGET="$COMMAND"
    checkout="$(checkout_of "$COMMAND")"

    if is_layout_link "$COMMAND"; then
        # Already the layout: the binary is replaced behind the link, which is left alone
        TARGET="$HOME_BINARY" LINK="$COMMAND"
        if is_hm2 "$HOME_BINARY"; then
            REPLACING="hm2"
            CURRENT_BINARY="$HOME_BINARY"
        fi
    elif [ -n "$checkout" ] && is_legacy "$COMMAND" "$checkout"; then
        LEGACY_CHECKOUT="$checkout"
        REPLACING="legacy"
    elif [ -n "$checkout" ]; then
        # A contributor's own build: replaced like any link, and the checkout is theirs, not 1.x
        SOURCE_CHECKOUT="$checkout"
        REPLACING="source"
    elif is_hm2 "$COMMAND"; then
        REPLACING="hm2"
        CURRENT_BINARY="$COMMAND"
    elif [ -e "$COMMAND" ] || [ -L "$COMMAND" ]; then
        die "$COMMAND exists and is not an hm this installer can replace; it was left as it is. Remove it, or install elsewhere: --dir <directory>"
    fi

    # A directory that is not the user's gets the link, and the binary goes where updates reach it.
    # One named with --dir or HM_INSTALL_DIR gets the binary itself, written with sudo, as asked.
    if [ -z "$LINK" ] && [ -z "$INSTALL_DIR" ] && ! writable_dir "$dir"; then
        TARGET="$HOME_BINARY" LINK="$COMMAND" MAKE_LINK=1
    fi

    if [ "$TARGET" == "$HOME_BINARY" ] && { [ -e "$HOME_BINARY" ] || [ -L "$HOME_BINARY" ]; } && ! is_hm2 "$HOME_BINARY"; then
        die "$HOME_BINARY exists and is not an hm this installer can replace; it was left as it is. Remove it, or install elsewhere: --dir <directory>"
    fi

    # A 1.x checkout at ~/hm gets the notice even when its link was elsewhere or already gone
    if [ -z "$LEGACY_CHECKOUT" ] && [ -d "$HOME/hm/.git" ] && { [ -e "$HOME/hm/bin/run" ] || [ -d "$HOME/hm/console" ]; }; then
        LEGACY_CHECKOUT="$HOME/hm"
    fi
}

install_hm() {
    # kept: the binary in use stays, nothing is downloaded; newer: it is newer than the latest release
    local version current="" dir source="" kept=false newer=false
    CHANGES=()
    header

    if [ -n "$VERSION" ]; then
        version="${VERSION#v}"
        is_version "$version" || die "'$VERSION' is not a release version (X.Y.Z or X.Y.Z-rc.N): this installer installs releases, and a branch has no binary. Nothing was installed.
To run a branch, build it from source (Go needed): git clone -b $VERSION https://github.com/$REPOSITORY.git && cd hiberus-dockergento && go build -o hm ./cmd/hm" 2
        [ "$(major_of "$version")" -ge 2 ] || refuse_shell_version "$version"
    fi

    choose_target
    dir="$(dirname "$COMMAND")"

    if [ -z "${version:-}" ]; then
        # In this shell rather than in $(...), so that a die inside it stops the spinner, which is
        # this shell's child, before its message
        spinner_start "Looking up the latest release"
        latest_version >"$WORK/version"
        version="$(cat "$WORK/version")"
        spinner_stop
        if [ -n "$FANCY" ]; then
            if [[ "$version" == *-* ]]; then step "Latest pre-release: $version"; else step "Latest release: $version"; fi
        fi
    fi

    [ -n "$CURRENT_BINARY" ] && current="$(version_of "$CURRENT_BINARY")"

    # The binary in use is kept when it is the one wanted; it is still moved behind a link when its
    # directory is not the user's, so that the next update needs no root
    if [ -n "$current" ] && [ "$current" == "$version" ]; then
        say "${green}hm $version is already installed at $COMMAND; it is up to date.${reset}"
        source="$CURRENT_BINARY" kept=true
    elif [ -n "$current" ] && [ -z "$VERSION" ] && version_gt "$current" "$version"; then
        say "${green}hm $current at $COMMAND is newer than the latest release ($version); it was left as it is.${reset}"
        version="$current"
        source="$CURRENT_BINARY" kept=true newer=true
    else
        download "$version"
        if [ -n "$FANCY" ]; then
            # platform_asset is hm_<os>_<arch>: shown as <os>/<arch>
            local platform
            platform="$(platform_asset)"
            platform="${platform#hm_}"
            step "Downloaded hm $version for ${platform/_//}, checksum verified"
        fi

        if [ ! -d "$dir" ]; then
            if [[ "$dir" == "$HOME"/* ]]; then
                mkdir -p "$dir"
            else
                as_needed "$(dirname "$dir")" mkdir -p "$dir" || die "Could not create $dir. Nothing was changed."
            fi
        fi

        local replacement="the binary"
        [ -z "$MAKE_LINK" ] || replacement="a link to $HOME_BINARY"
        case "$REPLACING" in
            legacy) CHANGES+=("replaced the link $COMMAND -> $(readlink "$COMMAND") of the git-clone installation with $replacement") ;;
            source)
                # Quoted, so that the command still works pasted when the path has spaces
                CHANGES+=("replaced the link $COMMAND -> $(readlink "$COMMAND"), a build of the source checkout $SOURCE_CHECKOUT, with $replacement (to run your build again: ln -sf $(printf '%q' "$(readlink "$COMMAND")") $(printf '%q' "$COMMAND"))") ;;
            hm2) CHANGES+=("upgraded hm $current to $version") ;;
        esac
        source="$WORK/hm"
    fi

    if [ -n "$MAKE_LINK" ]; then
        if place_layout "$source"; then
            [ "$REPLACING" != "hm2" ] || CHANGES+=("moved the binary from $COMMAND to $HOME_BINARY")
        elif $kept; then
            # It was to move only so that updates need no root; where it is, it still works
            warn "Moving $COMMAND to $HOME_BINARY needs root, and sudo is not available or was refused; it was left where it is, so updates will still need sudo until this installer is run again with sudo."
            TARGET="$COMMAND" LINK="" MAKE_LINK=""
        else
            # Only a fresh install has somewhere else to go; anything already there stays as it was
            [ -z "$REPLACING" ] ||
                die "Could not write $COMMAND: it needs root, and sudo is not available or was refused. Nothing was changed. To install where you can write: --dir ~/.local/bin"
            warn "Linking $COMMAND needs root, and sudo is not available or was refused; hm goes to $HOME/.local/bin instead."
            COMMAND="$HOME/.local/bin/hm" TARGET="$HOME/.local/bin/hm" LINK="" MAKE_LINK=""
            dir="$HOME/.local/bin"
            if [ -e "$TARGET" ] || [ -L "$TARGET" ]; then
                die "$TARGET exists and is not an hm this installer can replace; it was left as it is. Remove it, or install elsewhere: --dir <directory>"
            fi
            mkdir -p "$dir"
            place "$source" "$TARGET"
        fi
    elif ! $kept; then
        [ "$TARGET" != "$HOME_BINARY" ] || mkdir -p "$HOME_BIN"
        place "$source" "$TARGET"
    fi

    # A newer binary left as it is was not installed by this installer, and gets no receipt unless
    # it was just moved behind a link
    if ! $newer || [ -n "$MAKE_LINK" ]; then
        write_receipt "$TARGET" "$version" "$LINK"
    fi
    if ! $kept; then
        if [ -n "$FANCY" ]; then step "Installed at $TARGET"; else say "${green}${bold}hm $version installed at $TARGET${reset}"; fi
    fi
    [ -z "$LINK" ] || CHANGES+=("the binary lives in $HOME_BINARY; $LINK links to it, so updates need no sudo")

    drop_legacy_completion
    install_completion "$TARGET"
    migrate_settings "$LEGACY_CHECKOUT"

    local change
    for change in ${CHANGES[@]+"${CHANGES[@]}"}; do step "$change"; done

    if ! on_path "$dir"; then
        warn "$dir is not in your PATH. Add it to your shell profile: export PATH=\"$dir:\$PATH\""
    elif [ "$(command -v hm 2>/dev/null || true)" != "$COMMAND" ] && [ -n "$(command -v hm 2>/dev/null || true)" ]; then
        warn "Another hm comes first in your PATH: $(command -v hm). Remove it so that $COMMAND is the one that runs."
    fi

    legacy_notice "$LEGACY_CHECKOUT"

    if [ -n "$FANCY" ]; then
        if [ -n "$LINK" ]; then box "hm $version is ready" "$LINK -> $HOME_BINARY"; else box "hm $version is ready" "$TARGET"; fi
        say ""
        say "  Next: open a new terminal (or source your profile), then run ${bold}${cyan}hm --help${reset}"
        say ""
    else
        say ""
        say "Open a new terminal (or source your profile) for the completion, then: hm --help"
    fi
}

# ------------------------------------------------------------------------------------- uninstalling

uninstall_hm() {
    local candidate path="" link="" profile checkout removed=false

    # The link first, so that it never points at a binary already gone
    for candidate in "$(receipt_field link)" "$(receipt_field path)" "${INSTALL_DIR:+$INSTALL_DIR/hm}" "$SYSTEM_BIN/hm" "$HOME/.local/bin/hm"; do
        if [ -n "$candidate" ] && is_layout_link "$candidate"; then
            link="$candidate"
            break
        fi
    done
    if [ -n "$link" ]; then
        as_needed "$(dirname "$link")" rm -f "$link"
        say "Removed the link $link"
        removed=true
        is_hm2 "$HOME_BINARY" && path="$HOME_BINARY"
    else
        for candidate in "$(receipt_field path)" "${INSTALL_DIR:+$INSTALL_DIR/hm}" "$SYSTEM_BIN/hm" "$HOME/.local/bin/hm" "$HOME_BINARY"; do
            [ -n "$candidate" ] || continue
            if is_hm2 "$candidate"; then
                path="$candidate"
                break
            fi
        done
    fi

    if [ -n "$path" ]; then
        as_needed "$(dirname "$path")" rm -f "$path"
        say "Removed $path"
        removed=true
        rmdir "$HOME_BIN" 2>/dev/null || true
    elif ! $removed; then
        say "No hm 2 binary installed by this installer was found; no binary was removed."
        checkout="$(checkout_of "$SYSTEM_BIN/hm")"
        if [ -n "$checkout" ]; then
            if is_legacy "$SYSTEM_BIN/hm" "$checkout"; then
                say "$SYSTEM_BIN/hm is a 1.x link and was left as it is."
            else
                say "$SYSTEM_BIN/hm is a link to a build of a source checkout and was left as it is."
            fi
        fi
    fi

    rm -f "$COMPLETION_DIR/hm.zsh" "$COMPLETION_DIR/hm.bash" "$RECEIPT"
    rmdir "$COMPLETION_DIR" 2>/dev/null || true

    for profile in "${PROFILES[@]}"; do
        if [ -f "$profile" ] && grep -q -F "$PROFILE_MARK" "$profile"; then
            edit_profile "$profile" "$PROFILE_MARK" && say "Removed the completion line from $profile"
        fi
    done

    if [ -d "$STATE_DIR" ]; then
        say "hm's own state (cache, settings) is kept in $STATE_DIR. To delete it as well: rm -rf $STATE_DIR"
    fi
    $removed && say "${green}hm was uninstalled.${reset}"
    return 0
}

# ------------------------------------------------------------------------------------------- main

main() {
    VERSION="${HM_VERSION:-}"
    PRE="${HM_PRE:-0}"
    INSTALL_DIR="${HM_INSTALL_DIR:-}"
    local action="install"

    while [ $# -gt 0 ]; do
        case "$1" in
            --version) [ $# -ge 2 ] || die "--version needs a version" 2; VERSION="$2"; shift ;;
            --version=*) VERSION="${1#*=}" ;;
            --pre) PRE=1 ;;
            --dir) [ $# -ge 2 ] || die "--dir needs a directory" 2; INSTALL_DIR="$2"; shift ;;
            --dir=*) INSTALL_DIR="${1#*=}" ;;
            --uninstall) action="uninstall" ;;
            -h | --help) usage; exit 0 ;;
            -*) die "Unknown option: $1 (see --help)" 2 ;;
            # A bare version, as the clone-based installer took it: bash -s -- 2.0.0
            *) VERSION="$1" ;;
        esac
        shift
    done

    [ "$PRE" == "true" ] && PRE=1
    INSTALL_DIR="${INSTALL_DIR%/}"
    if [ -n "$INSTALL_DIR" ] && [[ "$INSTALL_DIR" != /* ]]; then INSTALL_DIR="$PWD/$INSTALL_DIR"; fi

    WORK="$(mktemp -d "${TMPDIR:-/tmp}/hm-install.XXXXXX")"
    # The spinner first: interrupted, the installer must not leave it drawing over the terminal
    trap 'spinner_stop; rm -rf "$WORK"' EXIT

    if [ "$action" == "uninstall" ]; then
        uninstall_hm
    else
        install_hm
    fi
}

main "$@"
