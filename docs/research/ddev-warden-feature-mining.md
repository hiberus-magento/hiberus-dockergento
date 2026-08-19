# Investigación: qué robar de DDEV y Warden

> Estado: **investigación / catálogo de features candidatas**. Nada implementado.
> Premisa fijada por el departamento: **seguimos con Dockergento**. Este documento no
> evalúa migrar, sino qué funcionalidades de DDEV y Warden merece la pena incorporar.
> Relacionado: [git-worktrees.md](git-worktrees.md).

---

## 1. Por qué mirar precisamente a estas dos

- **Warden** es la referencia en el mundo Magento: mismo dominio que nosotros (Magento 1
  y 2, Adobe Commerce), mismo enfoque (CLI sobre Docker Compose), y lleva años resolviendo
  problemas que nosotros tenemos abiertos.
- **DDEV** viene del mundo Drupal/WordPress pero es, con diferencia, la herramienta con la
  superficie de comandos más madura y con un ecosistema de *add-ons* real.

Ninguna de las dos resuelve el problema de agentes en paralelo (ver
[git-worktrees.md](git-worktrees.md) §11), pero ambas tienen decenas de utilidades de
desarrollo diario que a Dockergento le faltan.

## 2. Superficie actual de Dockergento

40 comandos, centrados en el ciclo *setup → start → magento/composer → mysql → debug*:

```
ai-init ai-pull ai-reset bash cloud cloud-login compatibility composer config-env
copy-from-container copy-to-container create-project debug-off debug-on docker-compose
docker-stop-all down exec grunt install magento masquerade mysql mysqldump n98-magerun
npm purge rebuild restart set-host setup ssl start stop test-integration test-unit
transfer-db transfer-media update varnish-off varnish-on
```

Lo que **no** existe hoy: inventario de proyectos, introspección del entorno, snapshots,
diagnóstico, perfilado, compartición pública, gestión de servicios opcionales, limpieza
de recursos y salida estructurada.

## 3. Catálogo de features candidatas

Ordenado por valor para el departamento. "Hoy en hm" describe el hueco real.

### 3.1 Servicios globales compartidos y proxy — *la pieza que más desbloquea*

| | |
|---|---|
| **Referencia** | Warden `svc`: Traefik + dnsmasq + Portainer + CA de SSL, una sola vez para toda la máquina. DDEV: `ddev-router` equivalente. |
| **Hoy en hm** | Cada proyecto publica 80, 443, 3306, 9200, 8025, 5672, 15672. **Dos proyectos Dockergento no pueden estar arriba a la vez.** |
| **Propuesta** | `hm proxy up|down|status`: Traefik global en 80/443 + certificado wildcard; los proyectos dejan de publicar puertos y se enrutan por `Host`. |
| **Beneficio** | Multi-proyecto simultáneo (hoy imposible), fin de los conflictos de puertos, y **prerrequisito de los worktrees por agente**. |
| **Coste** | Medio-alto: toca plantilla compose, `ssl.sh`, `set-host.sh` y documentación. |

Complemento: **dnsmasq** para resolver `*.local` (o `*.test`) sin tocar `/etc/hosts` con
`sudo`, que es lo que hace hoy `set-host.sh`. Warden además firma un **certificado
wildcard** una sola vez (`warden sign-certificate`), frente a nuestro `mkcert` por dominio.

### 3.2 Introspección: `describe` y `list`

| | |
|---|---|
| **Referencia** | `ddev describe` (URLs, credenciales, puertos, versiones) y `ddev list` (todos los proyectos de la máquina y su estado). |
| **Hoy en hm** | Nada. Para saber la URL, el puerto de la BD o qué proyectos están arriba hay que leer ficheros o tirar de `docker ps`. |
| **Propuesta** | `hm describe [--json]` y `hm list [--json]`. |
| **Beneficio** | DX inmediata, soporte interno más rápido y **base para todo el trabajo con IA** (ver [ai-features.md](ai-features.md) §3). |
| **Coste** | Bajo. Es leer `properties.json`, `requirements.json` y `docker compose ps`. |

### 3.3 Snapshots de base de datos

| | |
|---|---|
| **Referencia** | `ddev snapshot` / `ddev snapshot restore`, con **backup en caliente** (`mariabackup`/`xtrabackup`) dentro del contenedor. `ddev stop --snapshot`, `ddev delete --snapshot` como red de seguridad. |
| **Hoy en hm** | `mysqldump` a fichero y `mysql -i` para volver. Lento en BDs grandes y sin gestión de versiones. |
| **Propuesta** | `hm db snapshot [nombre]`, `hm db restore [nombre]`, `hm db list`. |
| **Beneficio** | Rehacer un `setup:upgrade` fallido en segundos; probar migraciones; y es **la pieza clave para clonar entornos por worktree**. |
| **Coste** | Medio. La versión "volumen golden" está diseñada en [git-worktrees.md](git-worktrees.md) §4.2. |

### 3.4 `doctor`: diagnóstico real

| | |
|---|---|
| **Referencia** | `ddev utility diagnose`, `port-diagnose` (quién ocupa el puerto), `dockercheck`, `compose-config`, `tls-diagnose` (mkcert, almacén de confianza, HTTPS). |
| **Hoy en hm** | Un único mensaje genérico: *"Docker is not properly configured … execute hm setup"*, que sale igual si Docker está parado, si falta el fichero compose o si el YAML es inválido. |
| **Propuesta** | `hm doctor`: demonio Docker, versión de compose, puertos ocupados y por quién, validez del compose, certificados y confianza, entradas de `/etc/hosts`, espacio de volúmenes, estado de cada servicio. |
| **Beneficio** | Es probablemente **la mejora de soporte con mejor relación valor/esfuerzo** de toda la lista. |
| **Coste** | Bajo-medio. |

### 3.5 Compartir el entorno: `share` / tunnel

| | |
|---|---|
| **Referencia** | `ddev share` con ngrok/cloudflared. Warden tiene túnel SSH para clientes de BD. |
| **Hoy en hm** | Nada. |
| **Beneficio en Magento, que es mayor que en otros stacks** | Enseñar una tienda local a cliente o QA sin desplegar, y sobre todo **recibir webhooks reales** (pasarelas de pago, ERPs, marketplaces) contra el entorno local. |
| **Coste** | Bajo si se apoya en cloudflared. |

### 3.6 Perfilado: XHProf/XHGui y Blackfire

| | |
|---|---|
| **Referencia** | `ddev xhprof` / `ddev xhgui`, `ddev blackfire`; Warden lleva Blackfire desde v0.7 y comando propio. |
| **Hoy en hm** | Sólo Xdebug (`debug-on`/`debug-off`). |
| **Beneficio** | El rendimiento es *el* problema recurrente de Adobe Commerce. Tener perfilado a un comando de distancia cambia cómo se atacan las incidencias de performance. |
| **Coste** | Medio: imagen PHP con la extensión y un servicio de UI. |

### 3.7 Rendimiento de ficheros en macOS: Mutagen

| | |
|---|---|
| **Referencia** | Ambas usan **Mutagen**; Warden lo arranca automáticamente en `env up`. DDEV añade `mutagen status`/`logs`/`mutagen-diagnose`. |
| **Hoy en hm** | Volumen nombrado `workspace` + `copy-to-container`/`copy-from-container` y, en `composer install`, un ciclo de copiar-parar-copiar de vuelta con `docker cp`. Funciona, pero es lento y es la fuente de la mayoría de rarezas en Mac. |
| **Propuesta** | Evaluar Mutagen como estrategia alternativa en `docker-compose.dev.mac.yml`. |
| **Beneficio** | Sincronización bidireccional continua: se acaban los `copy-*` manuales y el vendor deja de estar desincronizado. |
| **Coste** | Alto (cambia el modelo de montaje en Mac) pero es la mejora de DX más grande que hay disponible para el equipo. |

### 3.8 Servicios opcionales y add-ons

| | |
|---|---|
| **Referencia** | `ddev add-on get/list/search/remove` con registro público: adminer, phpmyadmin, dbgate, redis-insight, cron, playwright, cypress, selenium, browsersync, minio (S3), grafana/opentelemetry, dblog y dbslow (log de consultas lentas)… |
| **Hoy en hm** | El stack es fijo. Existe `config/hm/commands` para comandos propios, pero no para *servicios*. |
| **Propuesta** | Empezar por perfiles opcionales dentro de la plantilla (`--with=adminer,selenium,cron`) antes de plantear un registro de add-ons. |
| **Beneficio** | Necesidades muy reales en Magento: **cron** (hoy no hay servicio de cron), **selenium/playwright** para MFTF y e2e, **adminer** para dar acceso a BD sin cliente instalado, **minio** para simular S3 de media. |
| **Coste** | Bajo por servicio, alto si se hace un registro completo. |

### 3.9 Bootstrap de proyecto al estilo Warden

| | |
|---|---|
| **Referencia** | `warden bootstrap` en el env-type de Magento 2: crea el proyecto, `composer install`, `setup:install` condicional según servicios activos (RabbitMQ, Redis, Varnish), configura base URLs, importa `app/etc/config.php`, modo developer, reindexa, y **genera usuario admin con contraseña aleatoria y 2FA de Google Authenticator con QR en el terminal**. Acepta `--clean-install`, `--db-dump`, `--skip-db-import`, `--meta-package`, `--meta-version`. |
| **Hoy en hm** | `hm setup` + `hm install` cubren buena parte, pero el usuario admin sale de `data/config.json` con **contraseña fija `Hiberus123`** y sin resolver el 2FA, que es el primer tropiezo de cualquiera que instale un Magento moderno. |
| **Propuesta** | Añadir a `install.sh`: credenciales aleatorias, alta y desactivación controlada de 2FA (o generación del QR), y flags `--clean-install` / `--db-dump`. |
| **Coste** | Bajo. Alto retorno en onboarding. |

### 3.10 Proveedores de `pull` / `push`

| | |
|---|---|
| **Referencia** | `ddev pull <proveedor>` y `ddev push`, con recetas declarativas para Pantheon, Acquia, Upsun, Lagoon y rsync. |
| **Hoy en hm** | `transfer-db`, `transfer-media` y `cloud`/`cloud-login`: la funcionalidad existe pero es ad-hoc e interactiva. |
| **Propuesta** | Formalizar un `hm pull` con proveedores declarativos: Adobe Commerce Cloud, SSH/rsync genérico, S3. |
| **Beneficio** | "Traerme el entorno de staging" en un comando reproducible, ejecutable también desde CI o desde un agente. |
| **Coste** | Medio. |

### 3.11 Higiene: `clean` y ciclo de vida seguro

| | |
|---|---|
| **Referencia** | `ddev clean` con *dry-run*; `stop --snapshot`, `delete --snapshot`. |
| **Hoy en hm** | `down -v` destruye volúmenes sin red de seguridad, y `docker-stop-all` para **todos** los contenedores de la máquina. |
| **Datos medidos** | En una máquina del equipo: **152 volúmenes, 69 GB**, 45 imágenes y 81 contenedores, con 8 GB reclamables sólo en imágenes. |
| **Propuesta** | `hm clean [--dry-run]` acotado a recursos de Dockergento y snapshot automático antes de destruir. |
| **Coste** | Bajo. |

### 3.12 Detalles pequeños con buen retorno

- **`hm launch`** — abrir la URL del proyecto en el navegador (`ddev launch`).
- **`hm logs [servicio] [-f]`** — hoy hay que ir a `docker compose logs` a mano.
- **Clientes de BD** — `ddev tableplus|sequelace|dbeaver` abren el cliente ya conectado.
- **Mailpit en lugar de Mailhog** — Warden migró en v0.15; **Mailhog está sin mantenimiento** y nuestra plantilla sigue usándolo.
- **Nombre de proyecto derivado del directorio** — DDEV lo tiene como opción global
  (`--omit-project-name-by-default`) precisamente **para que cada worktree sea un proyecto
  distinto sin configurar nada**. Encaja directo con nuestra línea de worktrees.
- **`hm version`** — hoy no hay forma de saber qué versión de Dockergento tienes.

## 4. Priorización propuesta

| Prioridad | Feature | Esfuerzo | Desbloquea |
|---|---|---|---|
| 1 | `hm doctor` | Bajo | Soporte interno, menos fricción de onboarding |
| 2 | `hm describe` / `hm list` (+ `--json`) | Bajo | DX y toda la línea de IA |
| 3 | `hm db snapshot` / `restore` | Medio | Worktrees, migraciones, red de seguridad |
| 4 | Proxy global + wildcard + dnsmasq | Medio-alto | **Multi-proyecto y multi-agente** |
| 5 | Mailpit, `logs`, `launch`, `version`, `clean` | Bajo | Calidad de vida |
| 6 | Servicios opcionales (cron, adminer, selenium) | Medio | Cron real, MFTF, e2e |
| 7 | `hm share` (túnel) | Bajo | Demos y webhooks reales |
| 8 | Perfilado (XHProf/Blackfire) | Medio | Incidencias de rendimiento |
| 9 | Mutagen en macOS | Alto | El mayor salto de DX del equipo |
| 10 | Proveedores `pull`/`push` | Medio | Reproducibilidad y CI |

## 5. Qué NO copiar

- **El registro de add-ons completo.** Mantener un ecosistema tiene un coste que no nos
  corresponde: con perfiles de servicios opcionales cubrimos el 90 % del valor.
- **Los env-types multi-framework** (Laravel, Symfony, Shopware…). Nuestro valor es lo
  contrario: estar 100 % enfocados en Magento 2 / Adobe Commerce.
- **Portainer como servicio global.** Aporta poco frente a Docker Desktop y añade una
  superficie que mantener.
