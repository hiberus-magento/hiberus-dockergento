# Hoja de ruta de la migración a Go

Punto de entrada para una sesión nueva sin contexto. Dice **dónde estamos, qué toca después y qué
reglas no se negocian**. El detalle histórico vive en `MIGRATION.md` y en
`openspec/changes/archive/`; este fichero no lo repite.

> Estado medido el **2026-11-19** sobre `release/2.0.0` (HEAD `496aa91` más la retirada de
> `grunt` y `update`; `cd0afc8` es lo último subido a origin).
> Los números caducan: vuelve a medirlos con los comandos de abajo antes de fiarte de ellos.

## Estado en una tabla

| Medida | Valor | Cómo comprobarlo |
|---|---|---|
| Comandos respondidos por el binario | **45 de 65** | `rg -n "Comandos en Go" MIGRATION.md` |
| Comandos aún en shell | **20** | `rg -c "\| shell \|" MIGRATION.md` |
| Ficheros en `console/commands/` | **20** | `fd -e sh . console/commands \| wc -l` |
| Gemelos de shell retirados | **45** | `rg -A3 '^RETIRED=' tests/unit/command_safety_test.sh` |
| Portados que **aún conservan** su `.sh` | **0** | pista A vacía |
| Tests Go / aserciones de shell | 602 / 586 | `DOCKER_HOST=unix:///nonexistent go test -json ./...` (pruebas y subpruebas que pasan) · `./tests/run.sh unit` |

## Qué hacer después, en orden

1. **Pista A — retirar los gemelos de shell que ya tienen port en Go.** Es lo más barato y lo que
   más desbloquea. Empieza por los pequeños y ya probados.
2. **Pista B — portar los 20 comandos que siguen en shell.** Uno por unidad, de menor a mayor.
3. **Pista C — defectos conocidos.** Ninguno bloquea; varios llevan cinco ciclos aplazados.

## Pista A — gemelos de shell pendientes de retirar

Vacía desde `exec` (ver `MIGRATION.md`). Si vuelve a haber un comando tabulado como `go` con su
`.sh` en el árbol, la regla para retirarlo está abajo, en las reglas que no se negocian, y sus
puntos ciegos en la pista C.

## Pista B — comandos aún en shell (20)

| Área | Comandos |
|---|---|
| environment | `launch`, `proxy`, `share`, `tui`, `tunnel` |
| tools | `compatibility`, `ssl`, `verify` |
| database | `db`, `transfer-db` |
| magento | **`install`** |
| ai | `ai-context`, `ai-doctor`, `ai-init`, `ai-pull`, `ai-reset`, `mcp`, `permissions` |
| files / versions | `transfer-media`, `switch` |

Ciclos ya identificados: `install-and-ssl-in-go`, `hosts-config-in-go`, y
que `Configuration` entre en la costura de `internal/cli`.

## Pista C — defectos conocidos

| Defecto | Dónde | Por qué importa |
|---|---|---|
| Disyunción de etiquetas que casa con cualquier contenedor sin `hm.project` cuando el nombre de proyecto es vacío | `dockergento/adapters/toolinfo/tooling.go` (~`:84-85`) | Arreglo: seleccionar la fuente antes de comparar, como `core.Container.Key()` |
| Sin derivación de `COMPOSE_PROJECT_NAME` desde el directorio | `dockergento/app/resolve.go` (~`:116`); `bin/run` sí lo hace (~`:100-103`) | El binario responde `name: ""` en proyectos sin la propiedad |
| Tercera copia de la resolución servicio→contenedor, con mensaje distinto | `dockergento/app/transfer.go` (`container`) | Converger cambia un mensaje al usuario: es decisión, no refactor |
| Fallo de `CopyFrom` bajo `--json` en macOS sale como texto por stderr | `internal/cli/wrappers.go` (`debugOn`) | Hueco estrecho, declarado |
| `tests/run.sh` con un argumento no reconocido corre **todo** en vez de fallar | `tests/run.sh` (~`:81-83`) | Un error tipográfico no se nota nunca |
| El número de métodos de la interfaz `commands` (18) solo está en prosa | `internal/cli/engine.go` | Nada lo vigila |
| `architecture/` puede describir comportamiento que el código no tiene | `architecture/*.md` | Ningún test vigila la documentación; §5.8.1 estuvo mal tres ciclos |
| Apartar un `.sh` no ve a los scripts que lo llaman directamente si ninguna suite los ejecuta, o si lo sustituyen por un stub | Pasó con `magento.sh`: lo llamaban `install.sh` y tres tareas, y apartarlo sólo rompió `worktree_test.sh:145`. Esas llamadas ya van por el binario; el punto ciego de la regla sigue | Antes de borrar, además de la suite: `rg '"\$COMMANDS_DIR"/<nombre>\.sh' console`, y la forma con llaves `${COMMANDS_DIR}/` — la que se escapó con `docker-compose.sh`; `no_dead_command_paths_test.sh` vigila ya las dos |
| Retirar un gemelo no ve los pasos que el código Go delega en `bin/run` | `set-host` pedía `magento config:set` por `legacy.Runner` y falló desde `cd0afc8` hasta `416f72b` | Antes de borrar, `rg 'Legacy.Run\(' dockergento/app` y comprobar qué ejecutor recibe cada uno en `dockergento.go` (`legacy.Runner` es `bin/run`; `selfrun.Runner`, el binario) |
| Apartar un `.sh` tampoco ve a las suites que ya fallan o se saltan sin daemon | `docker-compose.sh`: `go_passthrough_test.sh` lo comparaba y ya fallaba sin Docker, y `worktree_environments_test.sh` se salta entera; las dos le preguntaban a `bin/run docker-compose` | Antes de borrar, también `rg '<nombre>' tests` y leer lo que salga en suites que fallen o se salten en la línea base |
| La forma larga del `--force` propio de `setup` (regenerar los compose) nunca le llega: `--force` es global y se lo quedan `globals()` y `global_options.sh` | `internal/cli/globals.go`, `console/helpers/global_options.sh` | `-f` sí llega. Y `hm setup --force` desde un worktree levanta la protección, como en el shell. Decidir si `setup` pasa a ser transparente para su `--force` o se documenta `-f` |
| `go_database_test.sh` con la base de datos parada sólo mira el código y `ok: false` | `tests/integration/go_database_test.sh` | La comparación con el shell cubría también el tipo de error; afirmarlo exige ver la negativa con daemon |
| `clean` nunca recoge plantillas muertas solas: `Anything()` sólo cuenta entornos y entornos de rama | `dockergento/core/collection.go` (`Anything`); el shell decidía igual (`clean.sh:265`) | Con sólo plantillas que sobran, `clean --force` sale con 0 y no borra nada |
| `clean` borra los volúmenes de un entorno de rama muerto aunque la exploración los liste como no atribuibles | `dockergento/app/collect.go` (`removeByProject`); el shell hacía lo mismo | Ahora sólo los que Compose etiquetó con ese proyecto (no por prefijo, que alcanzaba a un `x_algo` vivo al recoger un `x` muerto), y un nombre vacío no casa con nada; que la exploración los dé por no atribuibles sigue siendo una contradicción de presentación |
| `write_from_docker-compose_templates.sh` sólo lo usa ya una prueba | `tests/integration/mail_catcher_test.sh` genera con él los compose que comprueba | Portar a Go (`core.RenderCompose`) sus reglas del capturador de correo y quitar la tarea |
| El paso `install` del `setup` de Go no pasa `-u` aunque se pida `--use-default` | `dockergento/app/setup.go` (`installStep` sólo lo pone `create-project`) | `install.sh` preguntará; el `setup.sh` del shell lo exportaba como `USE_DEFAULT_SETTINGS` |
| `set_magento_configs.sh` sólo lo usa ya una prueba | `tests/unit/properties_migration_test.sh` (`set_current_domain`) | Su par en Go, `Importer.rememberDomain`, no tiene prueba: portar la de recuperar el dominio y quitar la tarea |
| `dbeaver`/`sequelace`/`tableplus` dan por `project_not_configured` cualquier error de `Describe` que no sea una negativa | `internal/cli/db_client.go` (copiado de `describe.go`) | Un daemon inaccesible se presenta como proyecto mal configurado; decidir con `describe` a la vez |
| `cloud-login` ignora sus argumentos y escribe prosa en stdout también con `--json` | `internal/cli/cloud.go` | Como el shell; `config-env` ya niega los argumentos que ignoraría |
| `hm stop -t 5` toma `-t` y `5` por servicios | `internal/cli/orchestrate.go` (`stop`) | `stop.sh` pasaba las opciones a `docker compose stop`; el port no. `restart` las niega con 2: decidir entre pasarlas o negarlas |
| `docker-stop-all` con el daemon inaccesible responde «No containers running» y sale con 0 | `dockergento/app/orchestrate.go` (`StopEverything`, `containers, _ := o.Engine.Containers()`) | Heredado del shell (`docker ps -q 2>/dev/null`); un fallo de conexión se lee como éxito. `go_passthrough_test.sh` fija hoy ese comportamiento |
| `docker-stop-all` no interactivo para todos los contenedores sin preguntar | `go_passthrough_test.sh` lo ejecutaba con `HM_NON_INTERACTIVE=1` y el daemon accesible | Corregido en `a4927e6` (la prueba fija un `DOCKER_HOST` inexistente). Queda la regla: ninguna suite corre un comando de máquina sin cortar el daemon dentro de la propia prueba |

Ya resueltos (no reabrir): CI sin macOS, `gofmt` sin `./dockergento`, CI corriendo `tests/unit`
como "todo" — los tres en `d03d54c`. El aviso de worktree «los contenedores sirven el código de…»,
que el binario no daba para ninguno de los ocho comandos de código que responde, en `f03a5e4`.
El baile del vendor de `composer` en macOS desde un worktree sin entorno propio: se niega, con la
salida `hm --force composer …`, y desde `28560a5` lo hace el binario.

## Reglas que no se negocian

| Regla | Por qué |
|---|---|
| Commits en `release/2.0.0`, en inglés, Conventional Commits, **sin firma de IA** (ni `Co-Authored-By` ni `Claude-Session`) | Norma del proyecto |
| Push, PR y release los decide el humano | |
| Nunca ejecutar `hm debug-on`/`debug-off`, `masquerade`, `docker-stop-all` contra un proyecto real | Editan contenedores, reinician servicios o borran |
| Ningún test puede llegar al daemon real; hay proyectos ajenos levantados | Fija `DOCKER_HOST=unix:///nonexistent` cuando corras suites amplias, **también `go test ./...`**: `test/e2e` corre contra el Docker que encuentre (`e2e.NeedsDocker` sólo mira `docker info`) y crea y borra contenedores en él |
| Nunca escribir en el `/etc/hosts` real | |
| Para borrar un gemelo de shell: **apártalo, corre la suite entera, anota qué se rompe, restaura, repara, borra** | Tres comprobaciones textuales fallaron antes; detalle en `MIGRATION.md` |
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
