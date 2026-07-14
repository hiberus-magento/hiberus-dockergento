# Build & Publish — soporte Magento / Adobe Commerce 2.4.9

Guía de acciones manuales para construir y publicar las imágenes Docker
necesarias para el stack de Magento 2.4.9 en el registro `hiberusmagento`
de Docker Hub.

Stack objetivo de 2.4.9 (definido en `data/requirements.json`):

| Servicio  | Versión           | Imagen                        | ¿Build propio? |
|-----------|-------------------|-------------------------------|----------------|
| PHP       | 8.5 (bookworm)    | `hiberusmagento/php:8.5-bookworm`   | **Sí (nuevo)** |
| OpenSearch| 3                 | `hiberusmagento/search:3-opensearch`| **Sí (nuevo)** |
| RabbitMQ  | 4.2               | `hiberusmagento/rabbitmq:4.2`       | **Sí (nuevo)** |
| MariaDB   | 11.8              | `mariadb:11.8` (oficial)      | No             |
| Valkey    | 9                 | `valkey/valkey:9-alpine` (oficial) | No        |
| Nginx     | 1.18              | `hiberusmagento/nginx:1.18`   | No (ya existe) |
| Mailhog   | 1                 | `hiberusmagento/mailhog:1`    | No (ya existe) |
| Varnish   | 7.1               | `hiberusmagento/varnish:7.1`  | No (ya existe) |
| Hitch     | 1.7               | `hiberusmagento/hitch:1.7`    | No (ya existe) |

Solo hay que construir y publicar **3 imágenes nuevas** (PHP, OpenSearch,
RabbitMQ), que son las que tienen configuración específica de Hiberus.
MariaDB y Valkey se consumen directamente desde sus imágenes oficiales
(igual que ya se hace con `mariadb:11.4` en 2.4.8), por lo que no requieren
build ni publish.

> Todos los comandos se ejecutan desde la raíz del repositorio.
> Requiere estar autenticado: `docker login` con una cuenta con permisos de
> push sobre la organización `hiberusmagento`.

### Verificación de tags base en Docker Hub

Todas las imágenes base se comprobaron en Docker Hub el **13-jul-2026** y
existen y están activas:

| Imagen base                              | Estado | Notas |
|------------------------------------------|--------|-------|
| `php:8.5-fpm-bookworm`                    | ✅     | Apunta a PHP 8.5.8 (multi-arch: amd64/arm64/arm/386/…) |
| `opensearchproject/opensearch:3.1.0`      | ✅     | Minor que corre producción (verificado por SSH); rango 3.x disponible 3.0.0 → 3.7.0 |
| `rabbitmq:4.2-management-alpine`          | ✅     | Multi-arch (amd64/arm64/arm/386/…) |
| `mariadb:11.8`                            | ✅     | Imagen oficial, consumo directo |
| `valkey/valkey:9-alpine`                  | ✅     | Oficial; arquitecturas amd64, arm64, arm/v7, ppc64le |

---

## 0. Entorno de build (macOS Apple Silicon + Colima + Rosetta)

Las imágenes se publican como **manifiesto multi-arquitectura**
(`linux/amd64` + `linux/arm64`): los servidores/CI son amd64, pero los
equipos de desarrollo (MacBook Apple Silicon) son arm64. Publicar solo la
arquitectura del portátil rompería en producción.

Entorno de referencia real: macOS Sequoia (Apple Silicon) + **Colima** con
backend `vz` + CLI de Docker por Homebrew.

### Por qué NO se usa el build multi-arch "en un paso"

El flujo habitual (`docker buildx build --platform amd64,arm64 --push` con un
builder `docker-container`) **no funciona bien en este entorno**:

- Con emulación **QEMU** (`tonistiigi/binfmt`), la capa amd64 **segfaultea**
  al compilar (`python3` en apt, y `docker-php-ext-install` produciendo
  `cp: cannot stat 'modules/*'`). QEMU es inestable compilando C emulado.
- Con **Rosetta**, la emulación amd64 es estable, **pero** el builder
  `docker-container` (buildkit dentro de un contenedor) **no ve el handler de
  Rosetta**: `docker buildx inspect` solo anuncia `linux/arm64`. Rosetta solo
  la aprovecha el **daemon** de Colima.

Solución que sí funciona: usar el builder **`colima`** (driver `docker`, que
delega en el daemon = Rosetta) para construir **cada arquitectura por
separado**, y fusionarlas en un manifiesto con `docker buildx imagetools
create`.

### Preparación (una sola vez)

```bash
# 1. Plugin buildx (Homebrew NO lo coloca solo en la carpeta de plugins)
brew install docker-buildx
mkdir -p ~/.docker/cli-plugins
ln -sfn "$(brew --prefix)/opt/docker-buildx/bin/docker-buildx" ~/.docker/cli-plugins/docker-buildx
docker buildx version

# 2. Rosetta instalado en el host (si no lo estaba)
softwareupdate --install-rosetta --agree-to-license

# 3. Activar Rosetta en Colima. IMPORTANTE: el vmType ya es 'vz', así que esto
#    NO recrea la VM (no se pierden imágenes ni volúmenes); solo la reinicia.
colima stop
colima start --vz-rosetta          # opcional: --cpu 4 --memory 8
```

Comprobar que Rosetta emula amd64 en build (con el builder `colima`):

```bash
docker buildx build --builder colima --platform linux/amd64 --load \
  --no-cache --progress=plain -t rosetta-test - <<'EOF'
FROM alpine
RUN uname -m
EOF
# En la salida del RUN debe verse: x86_64
```

### Autenticación

```bash
docker login    # cuenta con permisos de push sobre hiberusmagento
```

---

## Método de build (por arquitectura + merge)

Patrón para cada imagen: construir amd64 (vía Rosetta) y arm64 (nativo) con el
builder `colima`, cargarlas (`--load`) y subir cada tag por arquitectura, y
luego fusionar en el tag final multi-arch. El driver `docker` **no admite
`--push`**, por eso se usa `--load` + `docker push`.

### 1. PHP 8.5

- **Contexto:** `Dockerfiles/php/8.5-bookworm/` — base `php:8.5-fpm-bookworm`
- **Tag final:** `hiberusmagento/php:8.5-bookworm`

```bash
docker buildx build --builder colima --platform linux/amd64 --load \
  -t hiberusmagento/php:8.5-bookworm-amd64 Dockerfiles/php/8.5-bookworm/
docker push hiberusmagento/php:8.5-bookworm-amd64

docker buildx build --builder colima --platform linux/arm64 --load \
  -t hiberusmagento/php:8.5-bookworm-arm64 Dockerfiles/php/8.5-bookworm/
docker push hiberusmagento/php:8.5-bookworm-arm64

docker buildx imagetools create -t hiberusmagento/php:8.5-bookworm \
  hiberusmagento/php:8.5-bookworm-amd64 hiberusmagento/php:8.5-bookworm-arm64
```

Puntos de atención:

- Imagen base `php:8.5-fpm-bookworm` **verificada en Docker Hub** (PHP 8.5.8).
- **Xdebug** fijado a **`xdebug-3.5.3`** (3.5.0 es la primera con soporte PHP
  8.5; 3.5.3 la última estable, 2026-06-08). Fuente:
  <https://xdebug.org/docs/compat>.
- conf/ y `docker-entrypoint.sh` heredados de 8.4-bookworm.
- **OPcache NO va en `docker-php-ext-install`** en 8.5: desde PHP 8.5 (RFC
  "Make OPcache a non-optional part of PHP") OPcache se compila estáticamente
  en el core y ya no existe `opcache.so`. Incluirlo rompe el build con
  `cp: cannot stat 'modules/*'`. Se configura solo con directivas `opcache.*`
  en el ini; no hay que instalarlo ni cargar `zend_extension=opcache.so`.
  Ref.: <https://wiki.php.net/rfc/make_opcache_required>,
  <https://github.com/docker-library/php/issues/1605>.
- Es el build **más lento**: compila extensiones amd64 vía Rosetta. Normal.

### 2. OpenSearch 3

- **Contexto:** `Dockerfiles/search/3-opensearch/` — base `opensearchproject/opensearch:3.1.0`
- **Tag final:** `hiberusmagento/search:3-opensearch`

```bash
docker buildx build --builder colima --platform linux/amd64 --load \
  -t hiberusmagento/search:3-opensearch-amd64 Dockerfiles/search/3-opensearch/
docker push hiberusmagento/search:3-opensearch-amd64

docker buildx build --builder colima --platform linux/arm64 --load \
  -t hiberusmagento/search:3-opensearch-arm64 Dockerfiles/search/3-opensearch/
docker push hiberusmagento/search:3-opensearch-arm64

docker buildx imagetools create -t hiberusmagento/search:3-opensearch \
  hiberusmagento/search:3-opensearch-amd64 hiberusmagento/search:3-opensearch-arm64
```

Puntos de atención:

- Producción (Cloud) corre OpenSearch **3.1.0** (verificado por SSH:
  `curl :9200` → `version.number: 3.1.0`); el `FROM` está en 3.1.0 para
  paridad dev↔producción. Si migras producción a otro minor 3.x, actualiza el
  `FROM`.
- **Certificación Adobe:** los System Requirements de 2.4.9 certifican
  OpenSearch a nivel **major = `3`** (no fijan minor). En Cloud
  `.magento/services.yaml` usa `type: opensearch:3` ("el minor no es
  necesario"); para infra gestionada indican "3.1 or latest available". El
  límite real es la compatibilidad del cliente `opensearch-php` de Magento.
- `conf/jvm.options` heredado; las opciones de GC ya van condicionadas por
  versión de JDK (`8-13:` / `14-:`), compatibles con el JDK de OpenSearch 3.

Cómo verificar la versión real de OpenSearch en Adobe Commerce (Cloud):

```bash
grep -A2 'opensearch:' .magento/services.yaml   # -> type: opensearch:3
magento-cloud relationships --property=opensearch
vendor/bin/ece-tools env:config:show services    # muestra type: opensearch:3
curl -XGET <opensearch-endpoint-ip>:9200         # version.number, p.ej. "3.1.0"
```

### 3. RabbitMQ 4.2

- **Contexto:** `Dockerfiles/rabbitmq/4.2/` — base `rabbitmq:4.2-management-alpine`
- **Tag final:** `hiberusmagento/rabbitmq:4.2`

```bash
docker buildx build --builder colima --platform linux/amd64 --load \
  -t hiberusmagento/rabbitmq:4.2-amd64 Dockerfiles/rabbitmq/4.2/
docker push hiberusmagento/rabbitmq:4.2-amd64

docker buildx build --builder colima --platform linux/arm64 --load \
  -t hiberusmagento/rabbitmq:4.2-arm64 Dockerfiles/rabbitmq/4.2/
docker push hiberusmagento/rabbitmq:4.2-arm64

docker buildx imagetools create -t hiberusmagento/rabbitmq:4.2 \
  hiberusmagento/rabbitmq:4.2-amd64 hiberusmagento/rabbitmq:4.2-arm64
```

Puntos de atención:

- Tag base `rabbitmq:4.2-management-alpine` **verificado en Docker Hub**.
- `conf/rabbitmq.conf` (`vm_memory_high_watermark.absolute = 1GB`) sigue
  siendo válido en RabbitMQ 4.x.
- OpenSearch y RabbitMQ no compilan (solo `COPY` de config): van rápido.

---

## Verificación de los manifiestos multi-arch

```bash
docker buildx imagetools inspect hiberusmagento/php:8.5-bookworm     # debe listar amd64 y arm64
docker buildx imagetools inspect hiberusmagento/search:3-opensearch
docker buildx imagetools inspect hiberusmagento/rabbitmq:4.2
```

Los tags intermedios `*-amd64` / `*-arm64` pueden borrarse del repo después,
dejando solo los tags finales.

---

## Troubleshooting de emulación (resumen)

| Síntoma | Causa | Solución |
|---------|-------|----------|
| `DEPRECATED: legacy builder` | `docker build` clásico sin BuildKit | Usar `docker buildx` |
| `unknown command: docker buildx` | Plugin buildx no instalado (CLI de Homebrew) | `brew install docker-buildx` + symlink en `~/.docker/cli-plugins` |
| `Segmentation fault` compilando en capa amd64 | Emulación **QEMU** inestable | Usar **Rosetta** (Colima `vz`) |
| `cp: cannot stat 'modules/*'` al instalar **opcache** en PHP 8.5 | OPcache es estático en el core desde 8.5, no hay `opcache.so` | Quitar `opcache` de `docker-php-ext-install` (afecta a cualquier arch) |
| `cp: cannot stat 'modules/*'` compilando otra extensión bajo amd64 emulado | cc1 amd64 se cae bajo QEMU | Usar **Rosetta** / build nativo |
| `buildx inspect` solo muestra `linux/arm64` con Rosetta | El builder `docker-container` no ve Rosetta | Construir con el builder **`colima`** (daemon) por arquitectura + `imagetools create` |
| `Cannot load builder default … /var/run/docker.sock` | El builder `default` apunta al socket estándar, no al de Colima | Usar `--builder colima` |
| El contenedor phpfpm sale con `Exited (2)` y `docker logs` muestra `cannot open /usr/local/bin/docker-entrypoint.sh: Permission denied` | El script/confs se copiaron sin permiso de lectura para el usuario `app` (`chmod +x` sobre 600 deja 711) | Copiar con `COPY --chmod=755` el entrypoint y `--chmod=644` los `.ini`/`.conf` |
| rabbitmq sale con `Exited (1)` y `eacces` / `failed_to_parse_configuration_file` sobre `/etc/rabbitmq/rabbitmq.conf` | `rabbitmq.conf` copiado sin permiso de lectura para el usuario `rabbitmq` | `COPY --chmod=644 conf/rabbitmq.conf` |
| opensearch (3.x) sale con `Exited (1)` y `NoClassDefFoundError: ...javaagent...AgentPolicy` | Se sobrescribió `config/jvm.options`, que en OpenSearch 3.x lleva el `-javaagent` del nuevo agente de seguridad | NO copiar un `jvm.options` propio; fijar heap por `OPENSEARCH_JAVA_OPTS` |

### Alternativa robusta para CI

Para publicaciones desatendidas, construir la capa amd64 en un **runner amd64
nativo** (sin emulación) y fusionar con la arm64 mediante
`docker buildx imagetools create`. Es el enfoque recomendado a medio plazo.

---

## Verificación post-publicación

Con las 3 imágenes publicadas, un `hm setup` sobre un proyecto 2.4.9
generará el `docker-compose.yml` con estas imágenes:

```
phpfpm    -> hiberusmagento/php:8.5-bookworm
db        -> mariadb:11.8
search    -> hiberusmagento/search:3-opensearch
redis     -> valkey/valkey:9-alpine        # slot 'redis', imagen Valkey
rabbitmq  -> hiberusmagento/rabbitmq:4.2
nginx     -> hiberusmagento/nginx:1.18
mailhog   -> hiberusmagento/mailhog:1
varnish   -> hiberusmagento/varnish:7.1
hitch     -> hiberusmagento/hitch:1.7
```

Comprueba disponibilidad de la versión y la tabla de compatibilidad:

```bash
hm compatibility          # 2.4.9 debe aparecer en la columna 2.4.x
```

> **Nota sobre Valkey:** se integra reutilizando el slot de servicio `redis`
> (host `redis`, flags `--*-redis-*` de `setup:install`). Valkey habla el
> protocolo Redis, por lo que no hace falta tocar `install.sh` ni la
> plantilla de docker-compose. El servicio interno se sigue llamando `redis`.
