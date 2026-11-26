#!/usr/bin/env bash
#
# That architecture/ does not cite what the code does not have.
#
# Nothing else watches those documents, and one section described behaviour the code had lost for
# three cycles. This is the cheap half of keeping them true: every repository path and every Go
# name they quote in backticks has to exist, so a rename or a removal fails here instead of
# leaving the document behind. Whether the prose around a name is still right is not checked.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

ROOT="$COMMAND_BIN_DIR"

# Cited on purpose as deleted: these must stay gone, not come back
RETIRED="bin/run console/ console/hm-completion.bash"

# The first segments that make a quoted path one of this repository's. Everything else with a
# slash is a path inside a Magento project or a container (`config/docker/properties.json`,
# `var/cache`, `/var/www/html`), which this repository cannot vouch for
REPOSITORY_ROOTS="cmd internal dockergento data docker-compose tests test architecture docs skills Dockerfiles changelogs specs .specify"

# The Go packages a qualified name may start with (`core.Refusal`, `composelib.Orchestrator.Exec`)
go_packages() {
    find "$ROOT/dockergento" "$ROOT/internal" -type d -name testdata -prune -o -type d -print | sed 's#.*/##' | sort -u
}

GO_PACKAGES=$(go_packages)

SCRATCH="$(mktemp -d)"
trap 'rm -rf "$SCRATCH"; hm_test_home_cleanup' EXIT

# Every Go source in one file, read once: a find per name made the suite take ten seconds
GO_SOURCES="$SCRATCH/sources.go"
find "$ROOT/cmd" "$ROOT/internal" "$ROOT/dockergento" "$ROOT/test" -name '*.go' -exec cat {} + > "$GO_SOURCES" 2>/dev/null

# A name is defined when some Go source declares it: a function or method, a type, a var or a
# const, or an indented one (a field, an interface method, a const in a block)
go_defined() {
    local name="$1"
    grep -Eq \
        "(func (\([^)]*\) )?$name(\(|\[)|type $name( |\[)|(var|const) $name( |=)|^[[:space:]]+$name([[:space:]]|\(|,|=|$))" "$GO_SOURCES"
}

is_retired() {
    local token="$1" retired
    for retired in $RETIRED; do
        [ "$token" == "$retired" ] && return 0
    done
    return 1
}

is_repository_path() {
    local first="${1%%/*}" root
    for root in $REPOSITORY_ROOTS; do
        [ "$first" == "$root" ] && return 0
    done
    return 1
}

is_go_package() {
    grep -qx "$1" <<< "$GO_PACKAGES"
}

# Checks one path: globs are expanded, a trailing slash means a directory
path_exists() {
    local path="$1" match
    if [[ "$path" == *"*"* ]]; then
        for match in $ROOT/$path; do
            [ -e "$match" ] && return 0
        done
        return 1
    fi
    if [[ "$path" == */ ]]; then
        [ -d "$ROOT/$path" ]
        return
    fi
    [ -e "$ROOT/$path" ]
}

# Prints `file: token` for every citation in the documents under $1 that names nothing
check_citations() {
    local directory="$1" document token bare name qualifier
    for document in "$directory"/*.md; do
        while IFS= read -r token; do
            token="${token#\`}"
            token="${token%\`}"
            [ -z "$token" ] && continue

            if is_retired "$token"; then
                path_exists "$token" && printf '%s: %s (cited as deleted, but it is there)\n' "$(basename "$document")" "$token"
                continue
            fi

            # Placeholders, commands and container paths are not citations of this repository
            case "$token" in
                *'<'* | *' '* | '~'* | /* | *'{'*) continue ;;
            esac

            # A path: under one of the repository's own roots, relative to dockergento/ when it
            # starts with adapters/, or a bare Go file or top-level document
            if [[ "$token" == */* ]]; then
                [[ "$token" == adapters/* ]] && token="dockergento/$token"
                is_repository_path "$token" || continue
                path_exists "$token" || printf '%s: %s\n' "$(basename "$document")" "$token"
                continue
            fi
            if [[ "$token" =~ ^[A-Za-z0-9_]+\.go$ ]]; then
                [ -n "$(find "$ROOT/cmd" "$ROOT/internal" "$ROOT/dockergento" -name "$token" | head -1)" ] ||
                    printf '%s: %s\n' "$(basename "$document")" "$token"
                continue
            fi
            if [[ "$token" =~ ^[A-Z][A-Z0-9_-]*\.md$ ]]; then
                [ -e "$ROOT/$token" ] || printf '%s: %s\n' "$(basename "$document")" "$token"
                continue
            fi

            # A Go name: `name`, `Name()`, `begin(label, ...)`, `Type.Method`, `pkg.Type.Method`
            bare="${token%%(*}"
            [[ "$bare" =~ ^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$ ]] || continue

            if [[ "$bare" == *.* ]]; then
                qualifier="${bare%%.*}"
                name="${bare##*.}"
                # `hm.project`, `xdebug.ini`: a dotted word whose head is neither a package nor
                # a Go type is not a Go name
                if ! is_go_package "$qualifier" && ! { [[ "$qualifier" =~ [A-Z] ]] && go_defined "$qualifier"; } &&
                    ! { [[ "$qualifier" =~ ^[a-z]+[A-Z] ]] && go_defined "$qualifier"; }; then
                    continue
                fi
                go_defined "$name" || printf '%s: %s\n' "$(basename "$document")" "$token"
                continue
            fi

            # Unqualified, only what reads as a Go name and nothing else: camelCase
            # (`resolveContainer`), several humps (`StopEverything`), or called (`Platform()`).
            # `Engine`, `RED` or `Microsoft` could be anything
            if [[ "$bare" =~ ^[a-z]+[A-Z][A-Za-z0-9]*$ ]] || [[ "$bare" =~ ^[A-Z][a-z0-9]+[A-Z][A-Za-z0-9]*$ ]] ||
                [[ "$token" == *'('* ]]; then
                go_defined "$bare" || printf '%s: %s\n' "$(basename "$document")" "$token"
            fi
        done < <(grep -o '`[^`]*`' "$document")
    done
}

# ---------------------------------------------------------------- it would see a drift
#
# The check is worth nothing if it passes on anything, so first a document that cites one of
# each kind of thing that does not exist, next to one of each kind that does.

FIXTURE="$SCRATCH/fixture"
mkdir -p "$FIXTURE"

cat > "$FIXTURE/fixture.md" << 'EOF'
Real: `internal/cli/run.go`, `adapters/composelib`, `dockergento/adapters/*`, `run.go`,
`MIGRATION.md`, `newEngine`, `core.Refusal`, `Engine.Stop`, `ExitUsage`, `Platform()`.
Not paths of this repository: `config/docker/properties.json`, `/var/www/html`, `hm.project`.
Gone: `internal/cli/vanished.go`, `adapters/vanished`, `dockergento/nothing*`, `vanished.go`,
`VANISHED.md`, `vanishedHelper`, `core.VanishedRefusal`, `Engine.Vanish`, `ExitVanished`,
`Vanish()`.
EOF

test_case "a citation of something that does not exist is reported, of every kind"
reported=$(check_citations "$FIXTURE" | sed 's/^fixture.md: //' | tr '\n' ' ')
assert_equals "internal/cli/vanished.go dockergento/adapters/vanished dockergento/nothing* vanished.go VANISHED.md vanishedHelper core.VanishedRefusal Engine.Vanish ExitVanished Vanish() " "$reported"

test_case "and a deleted path cited as deleted is reported when it comes back"
mkdir -p "$FIXTURE/root/bin" && touch "$FIXTURE/root/bin/run"
assert_equals "fixture.md: bin/run (cited as deleted, but it is there)" \
    "$(printf 'Gone: `bin/run`.\n' > "$FIXTURE/fixture.md" && ROOT="$FIXTURE/root" check_citations "$FIXTURE")"

# ---------------------------------------------------------------- and the documents are true

test_case "everything architecture/ cites in backticks exists"
assert_equals "" "$(check_citations "$ROOT/architecture")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
