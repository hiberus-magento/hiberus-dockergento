#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh

STABLE_BRANCH="main"
install_dir="$COMMAND_BIN_DIR"
target=""
list_only=false

#
# The installation must be a git checkout: that is how the installer sets it up, and it is
# what makes switching versions a git operation on a directory we already know
#
require_git_installation() {
    if ! git -C "$install_dir" rev-parse --git-dir >/dev/null 2>&1; then
        hm_fail "$HM_EXIT_ERROR" "not_a_git_installation" \
            "This is not a git installation of $COMMAND_TOOLNAME" \
            "Reinstall with the installer to be able to switch versions"
    fi
}

#
# Never discard someone's work. An automatic stash is an elegant way of losing things, so
# this refuses and says what is in the way.
#
# Only tracked changes count. Untracked files are the user's own —notes, editor config, AI
# skills— they are always there, and git allows a checkout with them present: it refuses by
# itself if the target would overwrite one.
#
require_clean_installation() {
    local changes
    changes=$(git -C "$install_dir" status --porcelain --untracked-files=no 2>/dev/null)

    if [ -z "$changes" ]; then
        return 0
    fi

    if is_json_output; then
        hm_fail "$HM_EXIT_BLOCKED" "dirty_installation" \
            "The installation directory has uncommitted changes, so the version was not changed" \
            "Commit or stash them in $install_dir"
    fi

    print_error "\nThe installation directory has uncommitted changes:\n\n"
    printf '%s\n' "$changes" | sed 's/^/    /'
    print_default "\nNothing was changed. Commit or stash them first in:\n  "
    print_code "$install_dir\n\n"

    exit "$HM_EXIT_BLOCKED"
}

#
# Without fetching, newly published versions are invisible
#
refresh_references() {
    git -C "$install_dir" fetch --tags --prune origin >/dev/null 2>&1 || true
}

#
# What is installed right now
#
current_reference() {
    local branch
    branch=$(git -C "$install_dir" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

    if [ "$branch" == "HEAD" ]; then
        git -C "$install_dir" describe --tags --abbrev=0 2>/dev/null || echo "unknown"
    else
        echo "$branch"
    fi
}

#
# Versions and branches available, marking the current one
#
list_references() {
    require_git_installation
    refresh_references

    local current versions branches
    current=$(current_reference)

    versions=$(git -C "$install_dir" tag --list --sort=-version:refname |
        grep -E '^[0-9]+\.[0-9]+\.[0-9]+' || true)
    branches=$(git -C "$install_dir" for-each-ref --format='%(refname:short)' refs/remotes/origin |
        sed 's|^origin/||' | grep -v '^HEAD$' || true)

    if is_json_output; then
        json_success "switch" "$(jq -n \
            --arg current "$current" \
            --arg versions "$versions" \
            --arg branches "$branches" \
            '{current: $current,
              versions: ($versions | split("\n") | map(select(length > 0))
                         | map({name: ., pre_release: (contains("-")), installed: (. == $current)})),
              branches: ($branches | split("\n") | map(select(length > 0)))}')"
        exit 0
    fi

    printf "\n"
    print_heading "Versions\n"
    printf '%s\n' "$versions" | sed '/^$/d' | while IFS= read -r version; do
        # A pre-release is worth labelling: it is there to be tried on purpose, not to be
        # picked by mistake by someone reading the list from the top
        local note=""
        case "$version" in
            *-*) note="pre-release" ;;
        esac

        if [ "$version" == "$current" ]; then
            printf "  %-22s" "$version"
            print_info "← installed"
            [ -n "$note" ] && printf "   %s" "$note"
            printf "\n"
        elif [ -n "$note" ]; then
            printf "  %-22s" "$version"
            print_warning "$note\n"
        else
            printf "  %s\n" "$version"
        fi
    done

    printf "\n"
    print_heading "Branches\n"
    printf '%s\n' "$branches" | sed '/^$/d' | while IFS= read -r branch; do
        if [ "$branch" == "$current" ]; then
            printf "  %s" "$branch"
            print_info "   ← installed\n"
        else
            printf "  %s\n" "$branch"
        fi
    done

    printf "\n"
    print_default "Switch with "
    print_code "$COMMAND_BIN_NAME switch <version|branch>"
    print_default ", or back to stable with "
    print_code "$COMMAND_BIN_NAME switch --stable"
    print_default "\n\n"
}

#
# Move the installation to a reference
#
switch_to() {
    local reference="$1"

    require_git_installation
    require_clean_installation
    refresh_references

    if ! git -C "$install_dir" rev-parse --verify --quiet "$reference" >/dev/null 2>&1 &&
       ! git -C "$install_dir" rev-parse --verify --quiet "origin/$reference" >/dev/null 2>&1; then
        hm_fail "$HM_EXIT_USAGE" "unknown_reference" \
            "There is no version or branch called '$reference'" \
            "$COMMAND_BIN_NAME switch --list"
    fi

    # A remote branch is checked out as a local branch so that `hm update` keeps working;
    # a tag stays detached on purpose, and `hm update` refuses there.
    local checkout_error
    if git -C "$install_dir" rev-parse --verify --quiet "refs/tags/$reference" >/dev/null 2>&1; then
        checkout_error=$(git -C "$install_dir" checkout --quiet "$reference" 2>&1) || true
    elif git -C "$install_dir" rev-parse --verify --quiet "refs/heads/$reference" >/dev/null 2>&1; then
        checkout_error=$(git -C "$install_dir" checkout --quiet "$reference" 2>&1) || true
    else
        checkout_error=$(git -C "$install_dir" checkout --quiet -B "$reference" \
            --track "origin/$reference" 2>&1) || true
    fi

    local now
    now=$(current_reference)

    if [ "$now" != "$reference" ] && [ -n "$checkout_error" ]; then
        hm_fail "$HM_EXIT_ERROR" "switch_failed" \
            "Could not switch to '$reference': $checkout_error" \
            "Check the installation directory: $install_dir"
    fi

    "$COMMAND_BIN_DIR"/generate_completion.sh >/dev/null 2>&1 || true

    local description
    description=$(git -C "$install_dir" describe --tags 2>/dev/null || echo "$now")

    if is_json_output; then
        json_success "switch" "$(jq -n --arg reference "$reference" --arg version "$description" \
            '{switched_to: $reference, version: $version}')"
        exit 0
    fi

    print_info "\nNow running $COMMAND_TOOLNAME $description\n"
    print_default "  What changed: "
    print_code "changelogs/\n"
    printf "\n"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --list)
            list_only=true
            shift
            ;;
        --stable)
            target="$STABLE_BRANCH"
            shift
            ;;
        -*)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" \
                "Unknown option: $1" \
                "$COMMAND_BIN_NAME switch --help"
            ;;
        *)
            target="$1"
            shift
            ;;
    esac
done

if $list_only || [ -z "$target" ]; then
    list_references
    exit 0
fi

switch_to "$target"
