#!/usr/bin/env bash

#
# Gathers everything that describes the current project.
#
# The information lives in four places: the project properties, the generated compose
# files, composer.lock and Docker itself. Only the last one needs containers, so the
# command still answers with the environment stopped, which is exactly when it is most
# often needed.
#
# Both Docker queries and the whole JSON assembly happen once. Spawning a jq process per
# field was the difference between answering in half a second and answering in four.
#

source "$HELPERS_DIR"/docker.sh
source "$HELPERS_DIR"/version.sh
source "$TASKS_DIR"/set_environment_labels.sh

HM_COMPOSE_CONFIG_CACHE=""

#
# Parsed compose configuration, resolved once per invocation
#
compose_config_json() {
    if [ -z "$HM_COMPOSE_CONFIG_CACHE" ]; then
        HM_COMPOSE_CONFIG_CACHE=$($DOCKER_COMPOSE config --format json 2>/dev/null || echo '{}')
    fi

    echo "$HM_COMPOSE_CONFIG_CACHE"
}

#
# Deploy mode, read from app/etc/env.php so that it works with the environment stopped
#
magento_deploy_mode() {
    local env_php="${MAGENTO_DIR:-.}/app/etc/env.php"

    if [ ! -f "$env_php" ]; then
        return 0
    fi

    grep -o "'MAGE_MODE'[[:space:]]*=>[[:space:]]*'[^']*'" "$env_php" 2>/dev/null |
        head -1 | sed "s/.*'\\([^']*\\)'$/\\1/" || true
}

#
# The admin's front name, read from app/etc/env.php.
#
# It is not always "admin": Magento generates a random one on install unless told otherwise, and
# a project that has one is a project where /admin is a 404. Same source as the deploy mode, and
# for the same reason — it answers with the environment stopped.
#
# The database can override this further, through admin/url/use_custom_path. Reading it would
# mean a running database and a query, which is not a price worth paying to build a URL; env.php
# is what `bin/magento info:adminuri` reports for every project that has not done that.
#
magento_admin_path() {
    local env_php="${MAGENTO_DIR:-.}/app/etc/env.php"
    local front_name=""

    if [ -f "$env_php" ]; then
        front_name=$(grep -o "'frontName'[[:space:]]*=>[[:space:]]*'[^']*'" "$env_php" 2>/dev/null |
            head -1 | sed "s/.*'\([^']*\)'$/\1/" || true)
    fi

    printf '%s' "${front_name:-admin}"
}

#
# Xdebug state inside the running php container
#
xdebug_state() {
    local container_id
    container_id=$(hm_service_container phpfpm 2>/dev/null)

    if [ -z "$container_id" ]; then
        echo "unknown"
        return 0
    fi

    if docker exec "$container_id" grep -q '^zend_extension' \
        /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini 2>/dev/null; then
        echo "on"
    else
        echo "off"
    fi
}

#
# State of every service of this project, as "service=state" lines
#
service_states() {
    hm_container_table |
        awk -F'|' -v project="${COMPOSE_PROJECT_NAME:-}" \
            '($5 == project || ($5 == "" && $3 == project)) && $4 != "" { print $4 "=" $2 }'
}

#
# Whole project description as a JSON object
#
collect_project_info() {
    local with_secrets="${1:-false}"

    # Prime both caches in this shell: the lookups below run in subshells, which would
    # otherwise throw the cache away and query Docker again for every field
    hm_load_container_table
    HM_COMPOSE_CONFIG_CACHE=$($DOCKER_COMPOSE config --format json 2>/dev/null || echo '{}')

    local running_count
    running_count=$(hm_environment_containers "${COMPOSE_PROJECT_NAME:-}" --running | sed '/^$/d' | wc -l | tr -d ' ')

    echo "$HM_COMPOSE_CONFIG_CACHE" | jq -c \
        --arg name "${COMPOSE_PROJECT_NAME:-}" \
        --arg root "${HM_ROOT:-$PWD}" \
        --arg worktree "${HM_WORKTREE:-}" \
        --arg domain "${DOMAIN:-}" \
        --arg magento_version "${HM_MAGENTO:-$(hm_magento_version)}" \
        --arg magento_mode "$(magento_deploy_mode)" \
        --arg magento_dir "${MAGENTO_DIR:-.}" \
        --arg workdir "${WORKDIR_PHP:-}" \
        --arg machine "${MACHINE:-}" \
        --arg hm_version "${HM_VERSION:-$(hm_installed_version)}" \
        --arg compose_version "$(get_docker_compose_version)" \
        --arg xdebug "$(xdebug_state)" \
        --arg admin_path "$(magento_admin_path)" \
        --arg compose_file "${DOCKER_COMPOSE_FILE:-}" \
        --arg compose_file_machine "${DOCKER_COMPOSE_FILE_MACHINE:-}" \
        --arg states "$(service_states)" \
        --argjson running "$running_count" \
        --argjson with_secrets "$with_secrets" \
    '
        ($states | split("\n") | map(select(length > 0) | split("=") | {key: .[0], value: .[1]}) | from_entries) as $state
        | (.services // {}) as $services
        | ($services | length) as $total
        | (if $running == 0 then "stopped" elif $running == $total then "running" else "partial" end) as $status
        | (
            def port($service; $target):
                ($services[$service].ports // [])
                | map(select((.target | tostring) == $target))
                | (.[0].published // "" | tostring);
            # The mail catcher is either of two services. `mail` is the key that does not depend
            # on which one, and `mailhog` is kept alongside it with the same value so that
            # anything already reading that key keeps working.
            (if $services["mailpit"] then "mailpit" else "mailhog" end) as $mail_service
            | (port($mail_service; "8025") | if . == "" then "" else "http://localhost:" + . end) as $mail_url
            | {
                base:     (if $domain == "" then "" else "https://" + $domain + "/" end),
                admin:    (if $domain == "" then "" else "https://" + $domain + "/" + $admin_path end),
                mail:     $mail_url,
                mailhog:  $mail_url,
                rabbitmq: (port("rabbitmq"; "15672") | if . == "" then "" else "http://localhost:" + . end),
                search:   (port("search";   "9200") | if . == "" then "" else "http://localhost:" + . end)
            }
        ) as $urls
        | {
            project: {
                name: $name, root: $root, worktree: $worktree, domain: $domain,
                status: $status, urls: $urls
            },
            magento: { version: $magento_version, mode: $magento_mode },
            services: [
                $services | to_entries[] | {
                    name: .key,
                    image: (.value.image // ""),
                    state: ($state[.key] // "not created"),
                    ports: [(.value.ports // [])[] | "\(.published // "")->\(.target // "")"]
                }
            ],
            paths: {
                magento_dir: $magento_dir,
                workdir: $workdir,
                strategy: (if $machine == "mac" then "named volume with selective binds" else "bind mount" end),
                compose_files: [$compose_file, $compose_file_machine]
            },
            tooling: {
                machine: $machine, hm_version: $hm_version,
                compose_version: $compose_version, xdebug: $xdebug
            }
        }
        + (if $with_secrets then
            { credentials: { database: {
                name:          ($services.db.environment.MYSQL_DATABASE // ""),
                user:          ($services.db.environment.MYSQL_USER // ""),
                password:      ($services.db.environment.MYSQL_PASSWORD // ""),
                root_password: ($services.db.environment.MYSQL_ROOT_PASSWORD // "")
            } } }
           else {} end)
    '
}
