# Hoja de ruta de hm 2.0

Punto de entrada para una sesión nueva sin contexto. Dice **dónde estamos, qué toca después y qué
reglas no se negocian**. El detalle histórico vive en `MIGRATION.md` y en
`openspec/changes/archive/`; este fichero no lo repite. Los principios que sigue todo cambio están
en `.specify/memory/constitution.md` (2.0.0).

> **La migración a Go está completada.** El binario responde los 66 comandos, la ayuda,
> `--version` y los comandos del proyecto; `bin/run` y el árbol `console/` ya no existen.
> Publicadas `2.0.0-rc.1` y `2.0.0-rc.2`: lo que queda es validarlas, publicar 2.0.0, fusionar en
> `main` y los defectos conocidos de la pista C.
>
> Estado medido el **2026-11-25** sobre `release/2.0.0` en `f02de97`.
> Los números caducan: vuelve a medirlos con los comandos de abajo antes de fiarte de ellos.

## Estado en una tabla

| Medida | Valor | Cómo comprobarlo |
|---|---|---|
| Comandos respondidos por el binario | **66 de 66** | `rg -n "Comandos en Go" MIGRATION.md` |
| Comandos aún en shell | **0** | `rg -c "\| shell \|" MIGRATION.md` |
| Ficheros de shell en `console/` | **0** (el autocompletado lo imprime `hm completion`) | `git ls-files console` |
| `bin/run` | **borrado** | `test -e bin/run` falla |
| Tests Go | todos pasan | `DOCKER_HOST=unix:///nonexistent go test ./...` · `./tests/run.sh unit` |
| Binarios publicados por CI | sólo para etiquetas `>= 2.0`, sin `v`, y sólo si sus tests pasan | `.github/workflows/release.yml` |
| Imágenes de Docker Hub | se construyen y publican en local, sin CI | `build-publish.sh` |
| `hm switch` y el instalador ante una 1.x | la rechazan | `docs/switch.md`, `installer.sh` |
| Etiqueta `2.0.0` | **sin crear**; la última es `2.0.0-rc.2` | `git tag --sort=-creatordate \| head -1` |

## Qué hacer después, en orden

1. ~~**Pista A** — retirar los gemelos de shell~~: cerrada, no queda ninguno.
2. ~~**Pista B** — portar los comandos que seguían en shell~~: cerrada, el binario responde los 66
   y `bin/run` está borrado (`MIGRATION.md`, «Cierre»).
3. **Etiquetar la release.** Candidatas publicadas: `2.0.0-rc.1` y `2.0.0-rc.2`. Falta validarlas y
   publicar `2.0.0` con el ritual de siempre (commit de release con `changelogs/v2.0.0.md` y
   etiqueta anotada sin `v`). CI pasa los tests y publica los binarios y la release de GitHub al
   empujar la etiqueta. Lo decide el humano.
4. **Fusionar `release/2.0.0` en `main`** por pull request, cuando la candidata esté validada.
5. **Pista C — defectos conocidos.** Ninguno bloquea la release; varios llevan cinco ciclos
   aplazados.

## Pista C — defectos conocidos

| Defecto | Dónde | Por qué importa |
|---|---|---|
| Disyunción de etiquetas que casa con cualquier contenedor sin `hm.project` cuando el nombre de proyecto es vacío | `dockergento/adapters/toolinfo/tooling.go` (~`:84-85`) | Arreglo: seleccionar la fuente antes de comparar, como `core.Container.Key()` |
| El binario no convierte el antiguo `config/docker/properties` en `properties.json` | Lo hacía `console/tasks/load_properties.sh` (`refactor_old_version`) en cada invocación, ya borrado | Un proyecto que sólo tenga el formato antiguo se ve como «no es un proyecto». Decidir si se porta o se documenta la conversión a mano |
| Un directorio cuyo nombre no deja nada admisible (`___`) responde `project_not_configured` | `dockergento/app/resolve.go` (`projectFrom`) | `bin/run` respondía `no_project_name` con la pista de renombrar o fijar la propiedad; el binario da una negativa más genérica |
| `hm _registry` ya cumplió su condición de desaparecer (nada en shell lee el registro) | `internal/cli/run.go`, `MIGRATION.md` | Sólo lo usa `go_passthrough_test.sh`; quitarlo exige mover esas aserciones |
| Tercera copia de la resolución servicio→contenedor, con mensaje distinto | `dockergento/app/transfer.go` (`container`) | Converger cambia un mensaje al usuario: es decisión, no refactor |
| Fallo de `CopyFrom` bajo `--json` en macOS sale como texto por stderr | `internal/cli/wrappers.go` (`debugOn`) | Hueco estrecho, declarado |
| `tests/run.sh` con un argumento no reconocido corre **todo** en vez de fallar | `tests/run.sh` (~`:81-83`) | Un error tipográfico no se nota nunca |
| El número de métodos de la interfaz `commands` (18) solo está en prosa | `internal/cli/engine.go` | Nada lo vigila |
| `architecture/` puede describir comportamiento que el código no tiene | `architecture/*.md` | Ningún test vigila la documentación; §5.8.1 estuvo mal tres ciclos. Reescrita para 2.0 el 2026-11-20 (sin fragmentos del shell), pero sigue sin nada que la compruebe |
| La forma larga del `--force` propio de `setup` (regenerar los compose) nunca le llega: `--force` es global y se lo queda `globals()` | `internal/cli/globals.go` | `-f` sí llega. Y `hm setup --force` desde un worktree levanta la protección, como en el shell. Decidir si `setup` pasa a ser transparente para su `--force` o se documenta `-f` |
| `go_database_test.sh` con la base de datos parada sólo mira el código y `ok: false` | `tests/integration/go_database_test.sh` | La comparación con el shell cubría también el tipo de error; afirmarlo exige ver la negativa con daemon |
| `clean` nunca recoge plantillas muertas solas: `Anything()` sólo cuenta entornos y entornos de rama | `dockergento/core/collection.go` (`Anything`); el shell decidía igual (`clean.sh:265`) | Con sólo plantillas que sobran, `clean --force` sale con 0 y no borra nada |
| `clean` borra los volúmenes de un entorno de rama muerto aunque la exploración los liste como no atribuibles | `dockergento/app/collect.go` (`removeByProject`); el shell hacía lo mismo | Ahora sólo los que Compose etiquetó con ese proyecto (no por prefijo, que alcanzaba a un `x_algo` vivo al recoger un `x` muerto), y un nombre vacío no casa con nada; que la exploración los dé por no atribuibles sigue siendo una contradicción de presentación |
| El paso `install` del `setup` de Go no pasa `-u` aunque se pida `--use-default` | `dockergento/app/setup.go` (`installStep` sólo lo pone `create-project`) | `install` preguntará; el `setup.sh` del shell lo exportaba como `USE_DEFAULT_SETTINGS` |
| `dbeaver`/`sequelace`/`tableplus` dan por `project_not_configured` cualquier error de `Describe` que no sea una negativa | `internal/cli/db_client.go` (copiado de `describe.go`) | Un daemon inaccesible se presenta como proyecto mal configurado; decidir con `describe` a la vez |
| `cloud-login` ignora sus argumentos y escribe prosa en stdout también con `--json` | `internal/cli/cloud.go` | Como el shell; `config-env` ya niega los argumentos que ignoraría |
| `grunt` mete el tema y el locale sin escapar en un `bash -c` del contenedor | `internal/cli/grunt.go`; el shell hacía lo mismo | Son los argumentos del propio usuario dentro de su contenedor; validarlos contra `Vendor/theme` y `xx_YY` lo cerraría |
| `hm stop -t 5` toma `-t` y `5` por servicios | `internal/cli/orchestrate.go` (`stop`) | `stop.sh` pasaba las opciones a `docker compose stop`; el port no. `restart` las niega con 2: decidir entre pasarlas o negarlas |
| `docker-stop-all` con el daemon inaccesible responde «No containers running» y sale con 0 | `dockergento/app/orchestrate.go` (`StopEverything`, `containers, _ := o.Engine.Containers()`) | Heredado del shell (`docker ps -q 2>/dev/null`); un fallo de conexión se lee como éxito. `go_passthrough_test.sh` fija hoy ese comportamiento |
| `docker-stop-all` no interactivo para todos los contenedores sin preguntar | `go_passthrough_test.sh` lo ejecutaba con `HM_NON_INTERACTIVE=1` y el daemon accesible | Corregido en `a4927e6` (la prueba fija un `DOCKER_HOST` inexistente). Queda la regla: ninguna suite corre un comando de máquina sin cortar el daemon dentro de la propia prueba |
| `update` sin Go: el aviso de la línea 74-80 de `update.go` y su prueba (R3-001 de `review-d173dd6c0796a3a6`) | `dockergento/update.go` | Aviso de revisión sin bloqueo; comprobar el caso que señala |
| `install`: restauración de `config.php` (R3-002 de `review-d173dd6c0796a3a6`) | `internal/cli/install.go:209-211` | Aviso de revisión sin bloqueo |
| La ayuda imprime literalmente secuencias que antes se interpretaban | `internal/cli/help.go:74` | Decidido así al quitar `bin/run`; revisar si alguna descripción las usaba |
| El instalador comprueba las dependencias después de clonar | `installer.sh:356` | Si falta una, deja un clon a medias; comprobarlas antes |
| `switch` decide con la rama local aunque esté desfasada respecto a `origin` | `dockergento/switch.go:57-58` | Una rama local vieja sin `cmd/hm` se rechazaría aunque `origin` ya lo tenga |
| Algunas suites escriben en el `~/.hm/cache` real (`validated-*-lab-*`) | `tests/integration/*` | Inofensivo, pero las pruebas deberían usar su propio `HOME` |

Ya resueltos (no reabrir): el CI en rojo permanente (fixture e2e sin el usuario `app`, jobs sin
límite de tiempo, runs viejos sin cancelar, release sin tests, actions en Node 20) y el workflow
`dockerhub-publish.yml`, retirado porque las imágenes se publican en local: en el commit que sigue a
`f02de97`. CI sin macOS, `gofmt` sin `./dockergento`, CI corriendo `tests/unit`
como "todo" — los tres en `d03d54c`. El aviso de worktree «los contenedores sirven el código de…»,
que el binario no daba para ninguno de los ocho comandos de código que responde, en `f03a5e4`.
El baile del vendor de `composer` en macOS desde un worktree sin entorno propio: se niega, con la
salida `hm --force composer …`, y desde `28560a5` lo hace el binario. La derivación de
`COMPOSE_PROJECT_NAME` desde el directorio, en `1094e18`. Los puntos ciegos de retirar un `.sh`
(llamadas directas, pasos delegados en `bin/run` por `legacy.Runner`, suites que ya fallaban) dejan
de existir con `d564d01`: no queda shell que retirar.

## Reglas que no se negocian

| Regla | Por qué |
|---|---|
| Commits en `release/2.0.0`, en inglés, Conventional Commits, **sin firma de IA** (ni `Co-Authored-By` ni `Claude-Session`) | Norma del proyecto |
| Push, PR y release los decide el humano | |
| Nunca ejecutar `hm debug-on`/`debug-off`, `masquerade`, `docker-stop-all` contra un proyecto real | Editan contenedores, reinician servicios o borran |
| Ningún test puede llegar al daemon real; hay proyectos ajenos levantados | Fija `DOCKER_HOST=unix:///nonexistent` cuando corras suites amplias, **también `go test ./...`**: `test/e2e` corre contra el Docker que encuentre (`e2e.NeedsDocker` sólo mira `docker info`) y crea y borra contenedores en él |
| Nunca escribir en el `/etc/hosts` real | |
| Para borrar código: **apártalo, corre la suite entera, anota qué se rompe, restaura, repara, borra** | Tres comprobaciones textuales fallaron antes al retirar shell; detalle en `MIGRATION.md` |
| Restaurar ficheros copiando un respaldo, **nunca** `git checkout <fichero>` | Revirtió una unidad entera sin commitear dejando `git status` limpio |
| `./tests/run.sh unit`, nunca `tests/unit` | El segundo corre todas las suites |
| Comprobar que una ruta está libre antes de escribir un fichero nuevo | Un `Write` machacó un `resolve.go` existente |
| Una mutación solo prueba algo si se aplica **y compila** | Si no, el verde o el rojo no dicen nada de la aserción |
| Para parsers con exclusión, un fixture que contenga **solo** lo excluido | Un fixture realista aprobó un parser incorrecto por el orden de las líneas |
| Las afirmaciones universales ("nunca", "ningún", "todos") se comprueban con un grep antes de escribirlas | Varias resultaron falsas |

## Mecánica de los ciclos

Los ciclos se han hecho con SDD (`/gentle-sdd-new <nombre>`), archivados en
`openspec/changes/archive/<fecha>-<nombre>/` con un `archive-report.md` que cuenta lo aprendido. Para
una retirada pequeña y sin decisiones abiertas se ha hecho directo, manteniendo el registro
(comprobación de mutación, nota del tercer paso en `MIGRATION.md`, enumeración del spec en
`openspec/specs/go-entrypoint/spec.md`).

## Al terminar un ciclo

- [ ] Actualiza la tabla de estado de arriba y la fila de `MIGRATION.md`.
- [ ] Quita de la pista A o B lo que se haya cerrado, y añade a la pista C lo que se haya descubierto.
- [ ] Cambia la fecha y el HEAD de la cabecera.
