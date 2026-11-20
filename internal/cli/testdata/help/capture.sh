#!/usr/bin/env bash
#
# Regenerates the help goldens from the real shell implementation. Run from anywhere, with bash:
#
#   bash internal/cli/testdata/help/capture.sh
#
# Captured on macOS, so the mac-only commands are in them; the Go tests pin the platform to match.
# Every page twice: without colour, and with colour forced onto the pipe.
#
set -euo pipefail

HERE="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd -P "$HERE/../../../.." && pwd)"
RUN="$ROOT/bin/run"
SCRATCH="$(mktemp -d)"
trap 'rm -rf "$SCRATCH"' EXIT

mkdir "$SCRATCH/empty"
cp -R "$HERE/project" "$SCRATCH/project"

unset HM_PROJECT_DIR HM_NO_COLOR NO_COLOR FORCE_COLOR CLICOLOR_FORCE

capture() {
    local dir="$1" out="$2"
    shift 2

    ( cd "$dir" && env NO_COLOR=1 TERM=xterm-256color "$RUN" "$@" > "$HERE/$out.txt" )
    ( cd "$dir" && env FORCE_COLOR=1 TERM=xterm-256color "$RUN" "$@" > "$HERE/$out.colour.txt" )
}

capture "$SCRATCH/empty" full --help
capture "$SCRATCH/project" project --help
capture "$SCRATCH/project" project-deploy deploy --help
capture "$SCRATCH/project" project-hello hello --help
capture "$SCRATCH/empty" unknown nonexistent --help

rm -f "$HERE"/commands/*.txt
for name in $(jq -r 'keys_unsorted[] | select(startswith("_") | not)' "$ROOT/data/command_descriptions.json"); do
    capture "$SCRATCH/empty" "commands/$name" "$name" --help
done
