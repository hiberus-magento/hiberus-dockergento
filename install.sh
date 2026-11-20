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

# Where a 1.x installation linked hm, and the default directory when it is writable. Overridable so
# the tests can stand in for it without root.
SYSTEM_BIN="${HM_SYSTEM_BIN:-/usr/local/bin}"

# The installer's own files: the shell completion and a receipt of what it installed
STATE_DIR="$HOME/.hm"
COMPLETION_DIR="$STATE_DIR/completion"
RECEIPT="$STATE_DIR/install-receipt"

# Marks the one profile line this installer owns, to replace or remove it and never add a second
PROFILE_MARK="# hm shell completion"

if [ -t 1 ]; then
    bold=$'\033[1m' green=$'\033[0;32m' orange=$'\033[0;33m' red=$'\033[0;31m' reset=$'\033[0m'
else
    bold="" green="" orange="" red="" reset=""
fi

say() { printf '%s\n' "$*"; }
warn() { printf '%s%s%s\n' "$orange" "$*" "$reset" >&2; }
die() {
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

# fetch <url> <file>: with curl, or wget when there is no curl. GITHUB_TOKEN goes to the API only,
# through a 0600 file removed afterwards. FETCH_STATUS is the HTTP status when curl could tell it.
fetch() {
    local url="$1" out="$2" header="" status=0
    local -a auth=()
    FETCH_STATUS=""

    if [ -n "${GITHUB_TOKEN:-}" ] && [[ "$url" == https://api.github.com/* ]]; then
        header="$(token_header)" || die "Could not write a private file for GITHUB_TOKEN in $WORK."
        auth=(-H "@$header")
    fi

    if command -v curl >/dev/null 2>&1; then
        FETCH_STATUS="$(curl -fsSL --retry 2 ${auth[@]+"${auth[@]}"} -o "$out" -w '%{http_code}' "$url")" || status=$?
    elif command -v wget >/dev/null 2>&1; then
        if [ -n "$header" ]; then
            # wget reads the header from a wgetrc of its own rather than from its arguments
            (umask 077 && printf 'header = %s\n' "$(cat "$header")" >"$header.wgetrc") &&
                wget -q --config="$header.wgetrc" -O "$out" "$url" || status=$?
        else
            wget -q -O "$out" "$url" || status=$?
        fi
    else
        die "Neither curl nor wget is installed; install one of them and run this again."
    fi

    [ -z "$header" ] || rm -f "$header" "$header.wgetrc"
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

    fetch "$DOWNLOADS/$version/$asset" "$WORK/hm" ||
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

# checkout_of <link>: the git checkout a link of the clone-based installer points into (1.x, or a
# 2.0 pre-release installed from git): <checkout>/bin/run or <checkout>/bin/hm. Nothing otherwise.
checkout_of() {
    local link="$1" target checkout
    [ -L "$link" ] || return 0
    target="$(readlink "$link")"
    case "$target" in
        */bin/run | */bin/hm) ;;
        *) return 0 ;;
    esac
    checkout="${target%/bin/*}"
    # bin/run is 1.x's entry point and nothing else's; a link to a bin/hm counts only inside a clone
    if [[ "$target" == */bin/run ]] || [ -e "$checkout/.git" ] || [ ! -e "$target" ]; then
        printf '%s' "$checkout"
    fi
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
            say "Writing to $dir needs root; sudo may ask for your password."
            SUDO_ANNOUNCED=1
        fi
        sudo "$@"
    else
        die "$dir is not writable and sudo is not available. Choose a directory of your own: --dir ~/.local/bin"
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
# and stays a link. The first edit keeps a copy of the original in <profile>.hm-backup. Whatever
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

# migrate_settings <checkout>: a 1.x installation kept the store and admin settings of `hm install`
# in <checkout>/data/config.json; hm 2 keeps them in ~/.hm/config.json. Copied once, when that is
# absent and the old file is a JSON object; never overwritten.
migrate_settings() {
    local source="$1/data/config.json" target="$STATE_DIR/config.json"
    [ -n "$1" ] && [ -f "$source" ] && [ ! -e "$target" ] && [ ! -L "$target" ] || return 0

    if ! tr -d ' \t\r\n' <"$source" | grep -q '^{.*}$'; then
        CHANGES+=("the 1.x install settings $source are not a JSON object and were not copied")
        return 0
    fi

    if mkdir -p "$STATE_DIR" && cp "$source" "$target"; then
        CHANGES+=("copied the 1.x install settings $source to $target")
    else
        CHANGES+=("could not copy the 1.x install settings $source to $target: copy it by hand")
    fi
}

write_receipt() {
    mkdir -p "$STATE_DIR"
    printf 'path=%s\nversion=%s\nsha256=%s\n' "$1" "$2" "$(sha256_of "$1")" >"$RECEIPT"
}

# ------------------------------------------------------------------------------------- installing

on_path() {
    case ":$PATH:" in *":$1:"*) return 0 ;; *) return 1 ;; esac
}

# Where the binary goes, and what is there now: sets TARGET, LEGACY_CHECKOUT and REPLACING
choose_target() {
    local dir checkout

    TARGET="" LEGACY_CHECKOUT="" REPLACING=""

    if [ -n "$INSTALL_DIR" ]; then
        dir="$INSTALL_DIR"
    elif [ -n "$(receipt_field path)" ] && is_hm2 "$(receipt_field path)"; then
        # Where this installer put hm last time, --dir included: running it again upgrades that one
        dir="$(dirname "$(receipt_field path)")"
    else
        # An hm already in the system directory — a 1.x link or an earlier hm 2 — is replaced at the
        # same path, so there is only ever one hm
        checkout="$(checkout_of "$SYSTEM_BIN/hm")"
        if [ -n "$checkout" ] || is_hm2 "$SYSTEM_BIN/hm"; then
            dir="$SYSTEM_BIN"
        elif is_hm2 "$HOME/.local/bin/hm"; then
            dir="$HOME/.local/bin"
        elif [ -d "$SYSTEM_BIN" ] && [ -w "$SYSTEM_BIN" ] && [ ! -e "$SYSTEM_BIN/hm" ] && [ ! -L "$SYSTEM_BIN/hm" ]; then
            dir="$SYSTEM_BIN"
        else
            dir="$HOME/.local/bin"
        fi
    fi

    TARGET="$dir/hm"
    LEGACY_CHECKOUT="$(checkout_of "$TARGET")"

    if [ -n "$LEGACY_CHECKOUT" ]; then
        REPLACING="legacy"
    elif is_hm2 "$TARGET"; then
        REPLACING="hm2"
    elif [ -e "$TARGET" ] || [ -L "$TARGET" ]; then
        die "$TARGET exists and is not an hm this installer can replace; it was left as it is. Remove it, or install elsewhere: --dir <directory>"
    fi

    # A 1.x checkout at ~/hm gets the notice even when its link was elsewhere or already gone
    if [ -z "$LEGACY_CHECKOUT" ] && [ -d "$HOME/hm/.git" ] && { [ -e "$HOME/hm/bin/run" ] || [ -d "$HOME/hm/console" ]; }; then
        LEGACY_CHECKOUT="$HOME/hm"
    fi
}

install_hm() {
    local version current="" dir
    CHANGES=()

    if [ -n "$VERSION" ]; then
        version="${VERSION#v}"
        is_version "$version" || die "'$VERSION' is not a release version (X.Y.Z or X.Y.Z-rc.N): this installer installs releases, and a branch has no binary. Nothing was installed.
To run a branch, build it from source (Go needed): git clone -b $VERSION https://github.com/$REPOSITORY.git && cd hiberus-dockergento && go build -o hm ./cmd/hm" 2
        [ "$(major_of "$version")" -ge 2 ] || refuse_shell_version "$version"
    fi

    choose_target
    dir="$(dirname "$TARGET")"

    [ -n "${version:-}" ] || version="$(latest_version)"

    [ "$REPLACING" == "hm2" ] && current="$(version_of "$TARGET")"

    if [ -n "$current" ] && [ "$current" == "$version" ]; then
        say "${green}hm $version is already installed at $TARGET; it is up to date.${reset}"
        write_receipt "$TARGET" "$version"
    elif [ -n "$current" ] && [ -z "$VERSION" ] && version_gt "$current" "$version"; then
        say "${green}hm $current at $TARGET is newer than the latest release ($version); it was left as it is.${reset}"
        version="$current"
    else
        download "$version"

        if [ ! -d "$dir" ]; then
            if [[ "$dir" == "$HOME"/* ]]; then mkdir -p "$dir"; else as_needed "$(dirname "$dir")" mkdir -p "$dir"; fi
        fi

        case "$REPLACING" in
            legacy) CHANGES+=("replaced the link $TARGET -> $(readlink "$TARGET") of the git-clone installation with the binary") ;;
            hm2) CHANGES+=("upgraded hm $current to $version") ;;
        esac

        place "$WORK/hm" "$TARGET"
        write_receipt "$TARGET" "$version"
        say "${green}${bold}hm $version installed at $TARGET${reset}"
    fi

    drop_legacy_completion
    install_completion "$TARGET"
    migrate_settings "$LEGACY_CHECKOUT"

    local change
    for change in ${CHANGES[@]+"${CHANGES[@]}"}; do say "  - $change"; done

    if ! on_path "$dir"; then
        warn "$dir is not in your PATH. Add it to your shell profile: export PATH=\"$dir:\$PATH\""
    elif [ "$(command -v hm 2>/dev/null || true)" != "$TARGET" ] && [ -n "$(command -v hm 2>/dev/null || true)" ]; then
        warn "Another hm comes first in your PATH: $(command -v hm). Remove it so that $TARGET is the one that runs."
    fi

    legacy_notice "$LEGACY_CHECKOUT"

    say ""
    say "Open a new terminal (or source your profile) for the completion, then: hm --help"
}

# ------------------------------------------------------------------------------------- uninstalling

uninstall_hm() {
    local candidate path="" profile removed=false

    for candidate in "$(receipt_field path)" "${INSTALL_DIR:+$INSTALL_DIR/hm}" "$SYSTEM_BIN/hm" "$HOME/.local/bin/hm"; do
        [ -n "$candidate" ] || continue
        if is_hm2 "$candidate"; then
            path="$candidate"
            break
        fi
    done

    if [ -n "$path" ]; then
        as_needed "$(dirname "$path")" rm -f "$path"
        say "Removed $path"
        removed=true
    else
        say "No hm 2 binary installed by this installer was found; no binary was removed."
        if [ -L "$SYSTEM_BIN/hm" ] && [ -n "$(checkout_of "$SYSTEM_BIN/hm")" ]; then
            say "$SYSTEM_BIN/hm is a 1.x link and was left as it is."
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
    trap 'rm -rf "$WORK"' EXIT

    if [ "$action" == "uninstall" ]; then
        uninstall_hm
    else
        install_hm
    fi
}

main "$@"
