# Hoja de ruta de hm 2.0

Punto de entrada para una sesión nueva sin contexto. Dice **dónde estamos, qué toca después y qué
reglas no se negocian**. El detalle histórico vive en `MIGRATION.md` y en
`openspec/changes/archive/`; este fichero no lo repite. Los principios que sigue todo cambio están
en `.specify/memory/constitution.md` (2.0.0).

> **La migración a Go está completada.** El binario responde los 67 comandos, la ayuda,
> `--version` y los comandos del proyecto; `bin/run` y el árbol `console/` ya no existen.
> Publicadas de `2.0.0-rc.1` a `2.0.0-rc.4`, y se seguirán publicando candidatas hasta ganar
> confianza: la 2.0.0 final y la fusión en `main` esperan a esa decisión. La pista C queda en los
> aplazados de abajo, y la evolución para agentes (MCP 2026-07-28, `hm status`, catálogo
> `hm --json`, envelopes de éxito, `--dry-run`, `--progress=jsonl`, permisos para Codex y
> OpenCode, plugin de Claude Code) está hecha: `odd/tasks/agentic.md` en la sesión que la hizo.
>
> Estado medido el **2026-11-25** sobre `release/2.0.0`.
> Los números caducan: vuelve a medirlos con los comandos de abajo antes de fiarte de ellos.

## Estado en una tabla

| Medida | Valor | Cómo comprobarlo |
|---|---|---|
| Comandos respondidos por el binario | **67 de 67** | `rg -n "Comandos en Go" MIGRATION.md` |
| Comandos aún en shell | **0** | `rg -c "\| shell \|" MIGRATION.md` |
| Ficheros de shell en `console/` | **0** (el autocompletado lo imprime `hm completion`) | `git ls-files console` |
| `bin/run` | **borrado** | `test -e bin/run` falla |
| Tests Go | todos pasan | `DOCKER_HOST=unix:///nonexistent go test ./...` · `./tests/run.sh unit` |
| Binarios publicados por CI | sólo para etiquetas `>= 2.0`, sin `v`, y sólo si sus tests pasan (Linux y macOS) |
| Dónde vive el binario | `~/.hm/bin/hm`, con un enlace en `/usr/local/bin`: `hm update` no pide `sudo` | `docs/installation.md` | `.github/workflows/release.yml` |
| Imágenes de Docker Hub | se construyen y publican en local, sin CI | `build-publish.sh` |
| `hm switch` y el instalador ante una 1.x | la rechazan | `docs/switch.md`, `installer.sh` |
| Etiqueta `2.0.0` | **sin crear**; la última es `2.0.0-rc.4` | `git tag --sort=-creatordate \| head -1` |

## Qué hacer después, en orden

1. ~~**Pista A** — retirar los gemelos de shell~~: cerrada, no queda ninguno.
2. ~~**Pista B** — portar los comandos que seguían en shell~~: cerrada, el binario responde los 66
   y `bin/run` está borrado (`MIGRATION.md`, «Cierre»).
3. **Etiquetar la release.** Candidatas publicadas: `2.0.0-rc.1` y `2.0.0-rc.2`. Falta validarlas y
   publicar `2.0.0` con el ritual de siempre (commit de release con `changelogs/v2.0.0.md` y
   etiqueta anotada sin `v`). CI pasa los tests y publica los binarios y la release de GitHub al
   empujar la etiqueta. Lo decide el humano.
4. **Fusionar `release/2.0.0` en `main`** por pull request, cuando la candidata esté validada.
5. ~~**Pista C — defectos conocidos**~~: sin ninguno abierto.

## Pista C — defectos conocidos

Ninguno abierto.

Cerrados para la rc.7: `architecture/` tiene quien la vigile (`tests/unit/architecture_citations_test.sh`:
toda ruta del repositorio y todo nombre de Go citado entre comillas invertidas existe), y
`go_database_test.sh` con la base de datos parada afirma también el tipo de la negativa
(`service_not_running`, con su pista).

Cerrados el 2026-11-25 (revisión `review-19842cf4645a4d43`, aprobada): `docker-stop-all` con el
daemon caído falla con `docker_failed`; `clean` recoge plantillas muertas solas; `___` responde
`no_project_name`; `debug-on` respeta `--json` al fallar la copia; `describe` y los clientes de base
de datos sólo llaman `project_not_configured` a una negativa (`app.NotConfigured`, una sola
construcción); `cloud-login` niega argumentos; `grunt` valida tema y locale; `tests/run.sh` niega un
argumento desconocido; `toolinfo` no casa nada con un nombre vacío; `setup -u` pasa `-u` a
`install`; las suites usan su propio HOME y caché. El contador de métodos de `engine.go` nunca
existió. Commits de `814d2ca` a `7929f29`.

Cerrados el 2026-11-25 (T8): el antiguo `config/docker/properties` se convierte una vez en `properties.json` y queda como `properties.bak`; `hm _registry` retirado; `setup --force` regenera los compose como `-f`; `clean` lista los volúmenes de un entorno de rama muerto bajo ese entorno; `transfer.go` usa la resolución servicio→contenedor compartida (que ya no niega un contenedor vivo listado tras uno parado).

Ya no aplican (retiradas el 2026-11-25): `docker-stop-all` no interactivo (corregido en `a4927e6`),
`update` sin Go (`update` descarga releases), la restauración de `config.php` de `install` (ya da
`config_restore_failed`), las secuencias literales de la ayuda (ninguna descripción tiene `%` ni
`\`), las dependencias de `installer.sh` (es un envoltorio de `install.sh`) y `switch` con la rama
local (`switch` instala releases, nunca ramas).

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
