#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

# ---------------------------------------------------------------------------
# Dynamic compatibility table
#
# Reads canonical version buckets from data/requirements.json and all version
# mappings from data/equivalent_versions.json to build the table at runtime.
# Adding a new version bucket + equivalences to those files is sufficient to
# have it appear here automatically — no changes to this script are needed.
# ---------------------------------------------------------------------------

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REQUIREMENTS_FILE="$REPO_ROOT/data/requirements.json"
EQUIVALENTS_FILE="$REPO_ROOT/data/equivalent_versions.json"

COL_WIDTH=10

# ---------------------------------------------------------------------------
# Collect bucket keys, split into 2.3.x and 2.4.x arrays
# ---------------------------------------------------------------------------
BUCKETS_23=()
BUCKETS_24=()

while IFS= read -r bucket; do
    case "$bucket" in
        2.3.*) BUCKETS_23+=("$bucket") ;;
        2.4.*) BUCKETS_24+=("$bucket") ;;
    esac
done < <(jq -r 'keys[]' "$REQUIREMENTS_FILE" | sort -V)

# ---------------------------------------------------------------------------
# For each bucket collect its sorted versions; store as space-separated string
# Sort: base version first (patch=-1), then by patch number ascending
# ---------------------------------------------------------------------------
get_bucket_versions() {
    local bucket="$1"
    jq -r --arg b "$bucket" \
        '[to_entries[] | select(.value == $b) | .key] |
         sort_by(
             . as $v |
             ($v | split("-p")) |
             if length == 2
             then [.[0], (.[1] | tonumber)]
             else [$v, -1]
             end
         ) | .[]' \
        "$EQUIVALENTS_FILE"
}

# Build per-bucket version lists (indexed parallel arrays)
BUCKET_NAMES=()
BUCKET_VER_LISTS=()   # each element: newline-separated versions
BUCKET_COUNTS=()

for bucket in "${BUCKETS_23[@]}" "${BUCKETS_24[@]}"; do
    BUCKET_NAMES+=("$bucket")
    versions_str=""
    count=0
    while IFS= read -r v; do
        versions_str+="${v}"$'\n'
        (( count++ )) || true
    done < <(get_bucket_versions "$bucket")
    BUCKET_VER_LISTS+=("$versions_str")
    BUCKET_COUNTS+=("$count")
done

# Determine max rows
MAX_ROWS=0
for n in "${BUCKET_COUNTS[@]}"; do
    (( n > MAX_ROWS )) && MAX_ROWS=$n || true
done

# ---------------------------------------------------------------------------
# Build indexed cell lookup: cell_at BUCKET_IDX ROW -> version string or ""
# ---------------------------------------------------------------------------
cell_at() {
    local idx="$1"
    local row="$2"
    local list="${BUCKET_VER_LISTS[$idx]}"
    local line=0
    while IFS= read -r v; do
        if (( line == row )); then
            printf '%s' "$v"
            return
        fi
        (( line++ )) || true
    done <<< "$list"
    printf ''
}

# ---------------------------------------------------------------------------
# Table geometry
# ---------------------------------------------------------------------------
num_23="${#BUCKETS_23[@]}"
num_24="${#BUCKETS_24[@]}"

# Section inner width = num_cols * COL_WIDTH + (num_cols-1) * 3 (for " | ")
section_width_23=$(( num_23 * COL_WIDTH + (num_23 > 0 ? (num_23 - 1) * 3 : 0) ))
section_width_24=$(( num_24 * COL_WIDTH + (num_24 > 0 ? (num_24 - 1) * 3 : 0) ))

# Total line width: "| " + section23 + " || " + section24 + " |"
total_width=$(( 2 + section_width_23 + 4 + section_width_24 + 2 ))
inner_width=$(( total_width - 2 ))

sep_line=$(printf '%*s' "$total_width" '' | tr ' ' '-')
title_sep=$(printf '%*s' "$total_width" '' | tr ' ' '=')

# Centered title
title="SUPPORTED MAGENTO VERSIONS"
title_pad=$(( (inner_width - ${#title}) / 2 ))
title_line=$(printf "|%*s%s%*s|" \
    "$title_pad" "" "$title" \
    "$(( inner_width - title_pad - ${#title} ))" "")

# Subheader
sub23="2.3.x"
sub24="2.4.x"
sub23_pad=$(( (section_width_23 - ${#sub23}) / 2 ))
sub24_pad=$(( (section_width_24 - ${#sub24}) / 2 ))
sub_line=$(printf "| %*s%s%*s || %*s%s%*s |" \
    "$sub23_pad" "" "$sub23" "$(( section_width_23 - sub23_pad - ${#sub23} ))" "" \
    "$sub24_pad" "" "$sub24" "$(( section_width_24 - sub24_pad - ${#sub24} ))" "")

# ---------------------------------------------------------------------------
# Assemble table
# ---------------------------------------------------------------------------
table="\n${title_sep}\n"
table+="${title_line}\n"
table+="${title_sep}\n"
table+="${sub_line}\n"
table+="${sep_line}\n"

for (( row=0; row<MAX_ROWS; row++ )); do
    line="| "
    # 2.3.x columns
    for (( ci=0; ci<num_23; ci++ )); do
        cell="$(cell_at "$ci" "$row")"
        line+="$(printf '%-'"${COL_WIDTH}"'s' "$cell")"
        if (( ci < num_23 - 1 )); then
            line+=" | "
        fi
    done
    line+=" || "
    # 2.4.x columns (offset in BUCKET_NAMES by num_23)
    for (( ci=0; ci<num_24; ci++ )); do
        bucket_idx=$(( num_23 + ci ))
        cell="$(cell_at "$bucket_idx" "$row")"
        line+="$(printf '%-'"${COL_WIDTH}"'s' "$cell")"
        if (( ci < num_24 - 1 )); then
            line+=" | "
        fi
    done
    line+=" |"
    table+="${line}\n"
done

table+="${sep_line}\n"

print_table "$table"
