#!/usr/bin/env bash

#
# Inventory of the Dockergento environments on this machine.
#
# Everything comes from container labels, so it works from any directory and does not
# need a registry file that could drift from reality.
#
# The whole inventory is aggregated in a single awk pass over one Docker query, and turned
# into JSON in a single jq pass. Per-environment lookups meant dozens of processes and
# seconds of wall time for a command meant to be refreshed often.
#

source "$HELPERS_DIR"/docker.sh

#
# One row per environment: name, root, worktree, magento, has_metadata, running, total
#
environment_rows() {
    hm_container_table | awk -F'|' '
        {
            key = ($5 != "" ? $5 : $3)

            if (key == "") {
                next
            }

            if ($5 != "") {
                metadata[key] = 1
            }

            if ($4 == "phpfpm") {
                dockergento[key] = 1
            }

            total[key]++

            if ($2 == "running") {
                running[key]++
            }

            if (root[key] == "") {
                root[key] = ($6 != "" ? $6 : $12)
            }

            if (worktree[key] == "") {
                worktree[key] = $7
            }

            if (magento[key] == "") {
                magento[key] = $8
            }
        }
        END {
            for (key in total) {
                if (!metadata[key] && !dockergento[key]) {
                    continue
                }

                printf "%s\037%s\037%s\037%s\037%s\037%d\037%d\n", \
                    key, root[key], worktree[key], magento[key], \
                    (metadata[key] ? "true" : "false"), running[key] + 0, total[key]
            }
        }
    ' | sort
}

#
# Every environment as a JSON array
#
collect_environments() {
    # Prime the cache in this shell: the lookups below all run in subshells
    hm_load_container_table

    # Fields are separated by the unit separator (0x1f), not by a tab: `read` treats
    # tabs as whitespace and collapses consecutive ones, which silently shifted every
    # field whenever an environment had no worktree or no recorded Magento version.
    #
    # The rows are piped straight into jq as well: rebuilding them through printf '%b'
    # would re-interpret escape sequences that appear inside the data itself.
    local name root worktree magento metadata running total branch orphan

    {
        while IFS=$'\037' read -r name root worktree magento metadata running total; do
            [ -z "$name" ] && continue

            branch=""
            orphan="false"

            if [ -n "$root" ]; then
                if [ -d "$root" ]; then
                    branch=$(git -C "$root" rev-parse --abbrev-ref HEAD 2>/dev/null || true)
                else
                    orphan="true"
                fi
            fi

            printf '%s\037%s\037%s\037%s\037%s\037%s\037%s\037%s\037%s\n' \
                "$name" "$root" "$worktree" "$magento" "$metadata" \
                "$running" "$total" "$branch" "$orphan"
        done <<< "$(environment_rows)"
    } | jq -R -s -c '
        split("\n")
        | map(select(length > 0) | split("\u001f") | {
            name: .[0],
            root: .[1],
            worktree: .[2],
            magento: .[3],
            has_metadata: (.[4] == "true"),
            containers: { running: (.[5] | tonumber), total: (.[6] | tonumber) },
            branch: .[7],
            orphan: (.[8] == "true"),
            status: (if (.[5] | tonumber) == 0 then "stopped"
                     elif (.[5] | tonumber) == (.[6] | tonumber) then "running"
                     else "partial" end)
        })'
}
