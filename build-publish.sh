#!/usr/bin/env bash
#
# build-publish.sh — Build & publish Hiberus Dockergento service images to Docker Hub.
#
# Auto-discovers images from Dockerfiles/<name>/<version>/Dockerfile and publishes
# them as hiberusmagento/<name>:<version>, multi-arch (amd64 + arm64).
#
# Key feature: by default it SKIPS versions already published on Docker Hub, so a
# plain run only builds what is new/missing. Use --force to rebuild anyway.
#
# Multi-arch strategy: builds each platform separately with the given buildx builder
# (loading into the local daemon), pushes an arch-suffixed tag, then stitches them
# into the final multi-arch tag with `docker buildx imagetools create`. This is the
# approach that works on macOS + Colima + Rosetta (the docker-container driver does
# not see Rosetta, and QEMU is unreliable compiling PHP).
#
# Requirements: docker + buildx, a working builder (default: colima), and `docker login`
# with push rights on the hiberusmagento organisation.
#
# Usage:
#   ./build-publish.sh [options] [filter ...]
#
# Filters (optional). Build only matching images:
#   php                     all php versions
#   php/8.5-bookworm        that exact image   (php:8.5-bookworm also accepted)
#
# Options:
#   -f, --force             build even if the tag already exists on Docker Hub
#   -n, --dry-run           show what would be built/skipped, do nothing
#   -b, --builder NAME      buildx builder to use            (default: colima)
#   -p, --platforms LIST    comma-separated platforms        (default: linux/amd64,linux/arm64)
#       --cleanup           delete the intermediate -<arch> tags after merging
#                           (needs DOCKERHUB_USER and DOCKERHUB_TOKEN env vars)
#   -h, --help              show this help
#
# Examples:
#   ./build-publish.sh                     # build every missing image
#   ./build-publish.sh php/8.5-bookworm    # just that one
#   ./build-publish.sh --force nginx/1.28  # rebuild nginx 1.28 even if published
#   ./build-publish.sh --dry-run           # audit what is missing on Docker Hub
#
set -euo pipefail

REGISTRY="hiberusmagento"
DOCKERFILES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/Dockerfiles"
BUILDER="${BUILDER:-colima}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
FORCE=false
DRY_RUN=false
CLEANUP=false
FILTERS=()

# ---- colours (fallback to plain if not a tty) --------------------------------
if [ -t 1 ]; then C_OK=$'\033[32m'; C_SKIP=$'\033[33m'; C_ERR=$'\033[31m'; C_INFO=$'\033[36m'; C_RST=$'\033[0m'
else C_OK=""; C_SKIP=""; C_ERR=""; C_INFO=""; C_RST=""; fi
info()  { echo "${C_INFO}==>${C_RST} $*"; }
ok()    { echo "${C_OK}✔${C_RST} $*"; }
skip()  { echo "${C_SKIP}∼ skip${C_RST} $*"; }
err()   { echo "${C_ERR}✘${C_RST} $*" >&2; }

usage() { sed -n '2,/^set -euo/p' "$0" | sed 's/^# \{0,1\}//; /^set -euo/d'; }

# ---- parse args --------------------------------------------------------------
while [ $# -gt 0 ]; do
    case "$1" in
        -f|--force)     FORCE=true ;;
        -n|--dry-run)   DRY_RUN=true ;;
        -b|--builder)   BUILDER="$2"; shift ;;
        -p|--platforms) PLATFORMS="$2"; shift ;;
        --cleanup)      CLEANUP=true ;;
        -h|--help)      usage; exit 0 ;;
        -*)             err "Unknown option: $1"; usage; exit 1 ;;
        *)              FILTERS+=("${1/:/\/}") ;;   # accept name:ver or name/ver
    esac
    shift
done

# ---- helpers -----------------------------------------------------------------

# True if the ref already exists in the registry.
image_exists() { docker buildx imagetools inspect "$1" >/dev/null 2>&1; }

# True if this name/version passes the (optional) filters.
matches_filter() {
    local name="$1" version="$2"
    [ ${#FILTERS[@]} -eq 0 ] && return 0
    local f
    for f in "${FILTERS[@]}"; do
        [ "$f" = "$name" ] && return 0
        [ "$f" = "$name/$version" ] && return 0
    done
    return 1
}

# Delete an intermediate tag via the Docker Hub API (best-effort).
delete_tag() {
    local repo="$1" tag="$2"
    if [ -z "${DOCKERHUB_USER:-}" ] || [ -z "${DOCKERHUB_TOKEN:-}" ]; then
        skip "cleanup $repo:$tag (set DOCKERHUB_USER/DOCKERHUB_TOKEN to enable)"; return 0
    fi
    local jwt
    jwt=$(curl -s -H "Content-Type: application/json" -X POST \
        -d "{\"username\":\"${DOCKERHUB_USER}\",\"password\":\"${DOCKERHUB_TOKEN}\"}" \
        https://hub.docker.com/v2/users/login/ | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
    [ -z "$jwt" ] && { err "cleanup: could not authenticate to Docker Hub"; return 1; }
    curl -s -o /dev/null -X DELETE -H "Authorization: JWT ${jwt}" \
        "https://hub.docker.com/v2/repositories/${REGISTRY}/${repo}/tags/${tag}/" \
        && ok "removed intermediate tag ${repo}:${tag}"
}

# Build every platform, push arch tags, merge into the final multi-arch tag.
build_and_publish() {
    local ctx="$1" name="$2" version="$3"
    local ref="${REGISTRY}/${name}:${version}"
    local arch_refs=() plat suffix aref

    IFS=',' read -ra plats <<< "$PLATFORMS"
    for plat in "${plats[@]}"; do
        suffix="${plat##*/}"                 # linux/amd64 -> amd64
        aref="${ref}-${suffix}"
        info "build ${aref}  (${plat})"
        docker buildx build --builder "$BUILDER" --platform "$plat" --load -t "$aref" "$ctx"
        docker push "$aref"
        arch_refs+=("$aref")
    done

    info "merge -> ${ref}"
    docker buildx imagetools create -t "$ref" "${arch_refs[@]}"
    ok "published ${ref}"

    if $CLEANUP; then
        for plat in "${plats[@]}"; do delete_tag "$name" "${version}-${plat##*/}"; done
    fi
}

# ---- preflight ---------------------------------------------------------------
command -v docker >/dev/null || { err "docker not found in PATH"; exit 1; }
docker buildx version >/dev/null 2>&1 || { err "docker buildx not available"; exit 1; }
if ! $DRY_RUN && ! docker buildx inspect "$BUILDER" >/dev/null 2>&1; then
    err "buildx builder '$BUILDER' not found. Create it or pass --builder."; exit 1
fi

info "registry=$REGISTRY  builder=$BUILDER  platforms=$PLATFORMS  force=$FORCE  dry-run=$DRY_RUN"
[ ${#FILTERS[@]} -gt 0 ] && info "filters: ${FILTERS[*]}"

# ---- main loop ---------------------------------------------------------------
built=0 skipped=0 failed=0
shopt -s nullglob
for dockerfile in "$DOCKERFILES_DIR"/*/*/Dockerfile; do
    dir="$(dirname "$dockerfile")"
    version="$(basename "$dir")"
    name="$(basename "$(dirname "$dir")")"
    ref="${REGISTRY}/${name}:${version}"

    matches_filter "$name" "$version" || continue

    if ! $FORCE && image_exists "$ref"; then
        skip "$ref (already published)"; skipped=$((skipped+1)); continue
    fi

    if $DRY_RUN; then
        info "would build $ref"; built=$((built+1)); continue
    fi

    if build_and_publish "$dir" "$name" "$version"; then
        built=$((built+1))
    else
        err "build failed: $ref"; failed=$((failed+1))
    fi
done

echo
info "done — built/queued: $built, skipped: $skipped, failed: $failed"
[ "$failed" -eq 0 ]
