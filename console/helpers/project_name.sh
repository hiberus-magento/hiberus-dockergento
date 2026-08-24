#!/usr/bin/env bash

#
# The project's identity.
#
# The compose project name is what names the containers, the network and — the part that
# matters — the volumes. Getting it wrong on an existing environment does not rename a label:
# it leaves that project's database attached to a name nobody asks for any more.
#
# So the order below is the whole design. A configured name always wins, and nothing derives
# or adjusts it. Derivation only happens where there was no name at all, which used to leave
# the CLI believing the name was the empty string while Compose had already derived one from
# the directory — two different truths about the same containers.
#

#
# hm_derive_project_name <directory> — the name Docker Compose would give this directory.
#
# The rule is copied from Compose, measured rather than assumed: lowercase, keep only
# [a-z0-9_-] (accented characters are *dropped*, not transliterated) and trim leading dashes
# and underscores. If nothing admissible is left, there is no name.
#
# Pure parameter expansion: this runs on every invocation, and it is a string.
#
hm_derive_project_name() {
    local name="${1##*/}"

    # Lowercase. Bash 3.2 has no ${var,,}, and `tr` would be a process on the hot path.
    local lower="" char
    local index=0
    while [ "$index" -lt "${#name}" ]; do
        char="${name:$index:1}"
        case "$char" in
            [A-Z]) lower="$lower$(_hm_lower_char "$char")" ;;
            *)     lower="$lower$char" ;;
        esac
        index=$((index + 1))
    done

    # Keep only what Compose keeps
    local clean=""
    index=0
    while [ "$index" -lt "${#lower}" ]; do
        char="${lower:$index:1}"
        case "$char" in
            [a-z0-9_-]) clean="$clean$char" ;;
        esac
        index=$((index + 1))
    done

    # Trim leading dashes and underscores
    while [ -n "$clean" ]; do
        case "$clean" in
            [-_]*) clean="${clean:1}" ;;
            *)     break ;;
        esac
    done

    printf '%s' "$clean"
}

#
# One character to lowercase, without a process
#
_hm_lower_char() {
    case "$1" in
        A) printf 'a' ;; B) printf 'b' ;; C) printf 'c' ;; D) printf 'd' ;;
        E) printf 'e' ;; F) printf 'f' ;; G) printf 'g' ;; H) printf 'h' ;;
        I) printf 'i' ;; J) printf 'j' ;; K) printf 'k' ;; L) printf 'l' ;;
        M) printf 'm' ;; N) printf 'n' ;; O) printf 'o' ;; P) printf 'p' ;;
        Q) printf 'q' ;; R) printf 'r' ;; S) printf 's' ;; T) printf 't' ;;
        U) printf 'u' ;; V) printf 'v' ;; W) printf 'w' ;; X) printf 'x' ;;
        Y) printf 'y' ;; Z) printf 'z' ;;
        *) printf '%s' "$1" ;;
    esac
}

#
# hm_resolve_project_name — assigns COMPOSE_PROJECT_NAME, never echoes it.
#
# Assigning rather than echoing keeps it out of a subshell, the same reason the worktree
# resolver assigns: a value computed inside `$( )` is lost to the caller.
#
hm_resolve_project_name() {
    if [ -n "${COMPOSE_PROJECT_NAME:-}" ]; then
        return 0
    fi

    # The root, not $PWD: `hm` runs from any subdirectory, and from a worktree the root is the
    # main checkout. Deriving from the current directory would give a different identity
    # depending on where the command was typed, which is the very failure this closes.
    local root="${HM_ROOT:-$PWD}"

    COMPOSE_PROJECT_NAME="$(hm_derive_project_name "$root")"
    export COMPOSE_PROJECT_NAME
}
