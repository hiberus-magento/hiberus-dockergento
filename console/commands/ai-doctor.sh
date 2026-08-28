#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/digest.sh
source "$TASKS_DIR"/ai_registration.sh

#
# What AI tooling this project has, where each piece came from, and whether it is still that.
#
# `hm ai-pull --force` used to go blind: it overwrote what it had installed and left nothing
# behind that could tell an updated skill from one somebody had edited. Worse, the checksum it
# recorded was computed with `sha256sum`, which does not exist on macOS and cannot digest a
# directory in any case — so half the machines recorded an empty string that looked like a
# checksum.
#
# This says, per resource: where it came from, when, at what version, and which of four states it
# is in. It changes nothing.
#

usage() {
    print_info "What AI tooling this project has, and whether it is up to date\n\n"
    print_default "  $COMMAND_BIN_NAME ai-doctor\n\n"
}

case "${1:-}" in
    "") ;;
    --help | -h) usage; exit 0 ;;
    *)
        hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $1" \
            "$COMMAND_BIN_NAME ai-doctor"
        ;;
esac

properties_file="$CUSTOM_PROPERTIES_DIR/ai-properties.json"

if [ ! -f "$properties_file" ]; then
    if is_json_output; then
        json_success "ai-doctor" '{"configured": false, "resources": []}'
        exit 0
    fi

    print_info "This project has no AI tooling configured.\n"
    print_default "  $COMMAND_BIN_NAME ai-init\n"
    exit 0
fi

registration=$(load_ai_registration) || exit "$HM_EXIT_PROJECT"
platforms=$(jq -r '.platforms // [] | join(" ")' "$properties_file")

#
# One line per installed resource: platform, kind, name, state, origin, version, installed
#
rows=""

for platform in $platforms; do
    for kind in skills agents; do
        directory=$(jq -r --arg p "$platform" --arg k "${kind}_dir" \
            '.platforms[$p][$k] // empty' "$DATA_DIR/ai-platforms.json")

        [ -z "$directory" ] && continue

        path="${HM_ROOT:-$PWD}/$directory"
        [ -d "$path" ] || continue

        for item in "$path"/*; do
            [ -e "$item" ] || continue
            case "$(basename "$item")" in .*) continue ;; esac

            name=$(basename "$item")

            # The installer records the path it was given, which is the one relative to the
            # project. Looking it up only by its absolute form found nothing and called
            # everything custom
            entry=$(printf '%s' "$registration" | jq -c --arg k "$kind" \
                --arg absolute "$item" --arg relative "$directory/$name" \
                '.[$k][$absolute] // .[$k][$relative] // {}')

            if [ "$entry" == "{}" ]; then
                #
                # Not ours. Said plainly, because the reason it matters is that `ai-pull` leaves
                # it alone: somebody should know which of their skills are their own.
                #
                rows="${rows}${platform}\t${kind}\t${name}\tcustom\t-\t-\t-\n"
                continue
            fi

            recorded=$(printf '%s' "$entry" | jq -r '.checksum // ""')
            origin=$(printf '%s' "$entry" | jq -r '.origin // "" | if . == "" then "-" else . end')
            version=$(printf '%s' "$entry" | jq -r '.version // "" | if . == "" then "-" else . end')
            installed=$(printf '%s' "$entry" | jq -r '.installed // "-" | .[0:10]')
            current=$(hm_digest_path "$item")

            state="current"

            if [ -z "$recorded" ]; then
                # Installed before there was anything worth recording
                state="unknown"
            elif [ "$recorded" != "$current" ]; then
                state="modified"
            fi

            #
            # For what came with the tool there is a source of truth on this machine, so "up to
            # date" is a question that can actually be answered offline
            #
            if [ -d "$COMMAND_BIN_DIR/$kind/$name" ] && [ "$state" == "current" ]; then
                if [ "$(hm_digest_path "$COMMAND_BIN_DIR/$kind/$name")" != "$current" ]; then
                    state="outdated"
                fi
            fi

            rows="${rows}${platform}\t${kind}\t${name}\t${state}\t${origin}\t${version}\t${installed}\n"
        done
    done
done

#
# Tracked, and no longer there
#
while IFS= read -r missing; do
    [ -z "$missing" ] && continue
    rows="${rows}-\tskills\t$(basename "$missing")\tmissing\t-\t-\t-\n"
done <<< "$(printf '%s' "$registration" | jq -r '.skills // {} | keys[]' 2>/dev/null |
    while IFS= read -r tracked; do [ -e "$tracked" ] || printf '%s\n' "$tracked"; done)"

if is_json_output; then
    json_success "ai-doctor" "$(printf "$rows" | jq -R -s '
        {configured: true,
         resources: (split("\n") | map(select(length > 0) | split("\t") |
            {platform: .[0], kind: .[1], name: .[2], state: .[3],
             origin: .[4], version: .[5], installed: .[6]}))}')"
    exit 0
fi

if [ -z "$rows" ]; then
    print_info "Nothing installed yet.\n"
    print_default "  $COMMAND_BIN_NAME ai-pull\n"
    exit 0
fi

printf '\n'
print_heading "AI tooling in this project\n\n"
printf '  %-10s %-30s %-9s %-24s %s\n' "PLATFORM" "NAME" "STATE" "FROM" "INSTALLED"
printf "$rows" | while IFS=$'\t' read -r platform kind name state origin version installed; do
    [ -z "$name" ] && continue
    printf '  %-10s %-30s %-9s %-24s %s\n' "$platform" "$name" "$state" \
        "$origin$([ "$version" != "-" ] && printf ' %s' "$version")" "$installed"
done
printf '\n'

outdated=$(printf "$rows" | grep -c 'outdated' || true)
modified=$(printf "$rows" | grep -c 'modified' || true)

[ "$outdated" -gt 0 ] && {
    print_warning_line "$outdated behind the version that came with this tool"
    print_default "  $COMMAND_BIN_NAME ai-pull\n"
}

[ "$modified" -gt 0 ] && {
    print_warning_line "$modified changed since they were installed"
    print_default "  Those edits are lost on the next $COMMAND_BIN_NAME ai-pull.\n"
    print_default "  Rename them to keep them: what the tool did not install, it does not touch.\n"
}

printf '\n'
