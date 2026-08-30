#!/usr/bin/env bash
#
# Ten branch environments at once.
#
# This is not a test and the suite does not collect it: it is the measurement that decides how
# urgently the tool has to be rebuilt. It answers two questions that cannot be answered by
# reading code.
#
#   1. Does the tool survive ten agents doing the same thing at the same time? Ten worktrees are
#      created in parallel and the registry is checked afterwards: whole files, no duplicates, no
#      locks left behind.
#   2. What does an environment cost? They are started one at a time and the memory of the Docker
#      VM is read after each, until either they all run or the machine says no.
#
# It creates a project of its own with the real images of the stack, so the containers are the
# ones a project runs. What it does not have is Magento: the numbers below are the floor of what
# an environment costs, never the ceiling.
#
# Everything it creates is removed at the end, including on interrupt.
#
set -uo pipefail

HM="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/bin/run"
WORKTREES="${1:-10}"
MEMORY_FLOOR_MB="${MEMORY_FLOOR_MB:-500}"

LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-lab"
DOMAIN="hm-lab.test"
PROXY_WAS_UP=false

export HM_WORKTREE_DIR="$LAB/registry"
export HM_LOCK_DIR="$LAB/locks"
export HM_STATE_DIR="$LAB/state"
export HM_SNAPSHOT_DIR="$LAB/snapshots"

say()  { printf '\033[1;37m%s\033[0m\n' "$*"; }
note() { printf '  %s\n' "$*"; }

cleanup() {
    say ""
    say "Limpiando"

    local name
    while IFS= read -r name; do
        [ -z "$name" ] && continue
        ( cd "$LAB/$PROJECT" && "$HM" worktree remove "$name" --force ) >/dev/null 2>&1
    done <<< "$(ls "$HM_WORKTREE_DIR/$PROJECT" 2>/dev/null | sed 's/\.json$//' | sort -u | grep -v '\.yml$')"

    ( cd "$LAB/$PROJECT" && docker compose -p "$PROJECT" down -v --remove-orphans ) >/dev/null 2>&1

    for container in $(docker ps -aq --filter "name=^${PROJECT}" 2>/dev/null); do
        docker rm -f "$container" >/dev/null 2>&1
    done
    for volume in $(docker volume ls -q 2>/dev/null | grep -E "^(${PROJECT}[-_]|hm-template-${PROJECT}-)" || true); do
        docker volume rm -f "$volume" >/dev/null 2>&1
    done

    $PROXY_WAS_UP || ( cd "$LAB/$PROJECT" 2>/dev/null && "$HM" proxy down ) >/dev/null 2>&1

    rm -rf "$LAB"
    note "hecho"
}
trap cleanup EXIT INT TERM

#
# The memory of the VM the containers run in, which on macOS is not the memory of the laptop.
# That distinction is the first thing this measures.
#
vm_memory_free_mb() {
    docker run --rm alpine:latest sh -c "awk '/MemAvailable/ {print int(\$2/1024)}' /proc/meminfo" 2>/dev/null
}

vm_memory_total_mb() {
    docker run --rm alpine:latest sh -c "awk '/MemTotal/ {print int(\$2/1024)}' /proc/meminfo" 2>/dev/null
}

elapsed_since() {
    printf '%ss' "$(( $(date +%s) - $1 ))"
}

# ---------------------------------------------------------------- the machine

say "La máquina"
note "docker            $(docker info --format '{{.ServerVersion}}' 2>/dev/null)"
note "CPU de la VM      $(docker info --format '{{.NCPU}}' 2>/dev/null)"
note "memoria de la VM  $(vm_memory_total_mb) MB total, $(vm_memory_free_mb) MB disponibles"
note "disco             $(df -h / | tail -1 | awk '{print $4}') libres en el anfitrión"

# ---------------------------------------------------------------- the project

say ""
say "Proyecto de partida"

mkdir -p "$LAB/$PROJECT/config/docker" "$LAB/$PROJECT/app/etc"

cat > "$LAB/$PROJECT/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: hiberusmagento/php:8.3-bookworm
    command: ["php-fpm"]
    volumes:
      - ./.:/var/www/html
    depends_on:
      - db
  nginx:
    # Built from Dockerfiles/nginx/1.28 rather than pulled: the published image still carries
    # `worker_connections 1048576`, which reserves 884 MB per container, and measuring that would
    # be measuring a defect rather than the architecture. Build it with:
    #   docker build -t hm-nginx-fixed:probe Dockerfiles/nginx/1.28
    image: ${HM_LAB_NGINX:-hm-nginx-fixed:probe}
    volumes:
      - ./.:/var/www/html
    depends_on:
      - phpfpm
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
      MYSQL_USER: magento
      MYSQL_PASSWORD: magento
    volumes:
      - dbdata:/var/lib/mysql
  search:
    image: hiberusmagento/search:1.2-opensearch
    environment:
      # `plugins.security.disabled` is not passed: the image already sets it, and OpenSearch
      # refuses to start when a setting arrives twice — with exit code 0 and the reason buried in
      # a log summary, which is how a container looks like it started and is not there a minute
      # later
      - discovery.type=single-node
      - OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m
    volumes:
      - searchdata:/usr/share/opensearch/data
  redis:
    image: hiberusmagento/redis:6.2
    volumes:
      - redisdata:/data
volumes:
  dbdata:
  searchdata:
  redisdata:
YAML
cp "$LAB/$PROJECT/docker-compose.yml" "$LAB/$PROJECT/docker-compose.dev.mac.yml"
cp "$LAB/$PROJECT/docker-compose.yml" "$LAB/$PROJECT/docker-compose.dev.linux.yml"

printf '{"MAGENTO_DIR": ".", "DOMAIN": "%s", "COMPOSE_PROJECT_NAME": "%s", "USE_PROXY": "true"}\n' \
    "$DOMAIN" "$PROJECT" > "$LAB/$PROJECT/config/docker/properties.json"
printf '{"packages": []}\n' > "$LAB/$PROJECT/composer.lock"
mkdir -p "$LAB/$PROJECT/vendor/composer"
printf '<?php return array();\n' > "$LAB/$PROJECT/vendor/composer/autoload_psr4.php"

( cd "$LAB/$PROJECT" && git init -q . && git add -A &&
  git -c user.email=lab@lab -c user.name=lab commit -qm inicial ) >/dev/null 2>&1

note "creado en $LAB/$PROJECT"

if docker ps --format '{{.Names}}' | grep -q '^hm-proxy$'; then
    PROXY_WAS_UP=true
    note "el proxy ya estaba levantado"
else
    ( cd "$LAB/$PROJECT" && "$HM" proxy up ) >/dev/null 2>&1
    note "proxy levantado para el experimento"
fi

started=$(date +%s)
( cd "$LAB/$PROJECT" && "$HM" start ) >/dev/null 2>&1
note "entorno principal arriba en $(elapsed_since "$started")"

waited=0
until ( cd "$LAB/$PROJECT" && docker compose -p "$PROJECT" exec -T db \
        mariadb -uroot -ppassword magento -e "SELECT 1" ) >/dev/null 2>&1 || [ "$waited" -gt 120 ]; do
    sleep 3
    waited=$((waited + 3))
done
note "base de datos lista tras $waited s"

started=$(date +%s)
( cd "$LAB/$PROJECT" && "$HM" db freeze --name=base ) >/dev/null 2>&1
note "plantilla de base de datos congelada en $(elapsed_since "$started")"

base_free=$(vm_memory_free_mb)
note "memoria disponible con el entorno principal arriba: ${base_free} MB"

# ---------------------------------------------------------------- part one: at once

say ""
say "1 · $WORKTREES altas simultáneas"

started=$(date +%s)
for i in $(seq 1 "$WORKTREES"); do
    ( cd "$LAB/$PROJECT" && "$HM" worktree add "rama-$i" --profile=agent --no-start ) \
        > "$LAB/add-$i.out" 2>&1 &
done
wait
add_elapsed=$(elapsed_since "$started")

registered=$(ls "$HM_WORKTREE_DIR/$PROJECT"/*.json 2>/dev/null | wc -l | tr -d ' ')
whole=0
for record in "$HM_WORKTREE_DIR/$PROJECT"/*.json; do
    [ -f "$record" ] || continue
    jq -e . "$record" >/dev/null 2>&1 && whole=$((whole + 1))
done
overlays=$(ls "$HM_WORKTREE_DIR/$PROJECT"/*.yml 2>/dev/null | wc -l | tr -d ' ')
projects=$(jq -r '.project' "$HM_WORKTREE_DIR/$PROJECT"/*.json 2>/dev/null | sort | uniq -d | wc -l | tr -d ' ')
git_worktrees=$(( $(git -C "$LAB/$PROJECT" worktree list | wc -l | tr -d ' ') - 1 ))
locks=$(ls "$HM_LOCK_DIR" 2>/dev/null | wc -l | tr -d ' ')
failed=$(grep -l '"ok": false' "$LAB"/add-*.out 2>/dev/null | wc -l | tr -d ' ')

note "tiempo total            $add_elapsed"
note "registros escritos      $registered de $WORKTREES"
note "registros íntegros      $whole de $registered"
note "overlays escritos       $overlays"
note "nombres duplicados      $projects"
note "worktrees en git        $git_worktrees"
note "locks sin liberar       $locks"
note "altas con error         $failed"

if [ "$failed" -gt 0 ]; then
    note ""
    note "primer error:"
    grep -h -m1 '"message"' "$LAB"/add-*.out 2>/dev/null | head -1 | sed 's/^/    /'
fi

# ---------------------------------------------------------------- part two: running

say ""
say "2 · arrancándolos de uno en uno"
printf '  %-8s %-10s %-12s %-14s %s\n' "ENTORNO" "ARRANQUE" "EN MARCHA" "MEM. LIBRE" "CONSUMO ACUM."

running=0
for i in $(seq 1 "$WORKTREES"); do
    free_before=$(vm_memory_free_mb)

    if [ "${free_before:-0}" -lt "$MEMORY_FLOOR_MB" ]; then
        note ""
        note "parando: quedan ${free_before} MB, por debajo del suelo de ${MEMORY_FLOOR_MB} MB"
        break
    fi

    started=$(date +%s)
    ( cd "$LAB/$PROJECT-worktrees/rama-$i" && "$HM" start ) >/dev/null 2>&1
    start_elapsed=$(elapsed_since "$started")

    running=$((running + 1))
    free_now=$(vm_memory_free_mb)

    # Of this environment specifically, and running rather than created: a container that exited
    # on startup costs nothing and proves nothing, and counting it would make the whole
    # measurement a lie
    up=$(docker ps --filter "label=com.docker.compose.project=$PROJECT-rama-$i" -q | wc -l | tr -d ' ')
    expected=$(docker ps -a --filter "label=com.docker.compose.project=$PROJECT-rama-$i" -q | wc -l | tr -d ' ')

    printf '  %-8s %-10s %-12s %-14s %s\n' \
        "rama-$i" "$start_elapsed" "$up/$expected" "${free_now} MB" "$(( base_free - free_now )) MB"
done

say ""
say "Resultado"
note "entornos de rama arrancados   $running de $WORKTREES"
note "contenedores en marcha        $(docker ps -q | wc -l | tr -d ' ')"
note "memoria libre al final        $(vm_memory_free_mb) MB de $(vm_memory_total_mb) MB"

if [ "$running" -gt 0 ]; then
    note "coste medio por entorno       $(( (base_free - $(vm_memory_free_mb)) / running )) MB"
fi

note ""
note "servicios que no llegaron a arrancar:"
docker ps -a --filter "name=^${PROJECT}" --filter "status=exited" --format '    {{.Names}}  ({{.Status}})' 2>/dev/null | head -12
[ -z "$(docker ps -a --filter "name=^${PROJECT}" --filter "status=exited" -q)" ] && note "    ninguno"

note ""
note "los diez contenedores que más consumen:"
docker stats --no-stream --format '    {{.Name}}  {{.MemUsage}}  {{.CPUPerc}}' 2>/dev/null |
    sort -k2 -h -r | head -10
