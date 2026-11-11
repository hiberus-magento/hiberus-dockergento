# Hoja de ruta de la migración a Go

Punto de entrada para una sesión nueva sin contexto. Dice **dónde estamos, qué toca después y qué
reglas no se negocian**. El detalle histórico vive en `MIGRATION.md` y en
`openspec/changes/archive/`; este fichero no lo repite.

> Estado medido el **2026-11-11** sobre `release/2.0.0` (HEAD `f03a5e4` más la retirada de
> `magento`, sin subir a origin).
> Los números caducan: vuelve a medirlos con los comandos de abajo antes de fiarte de ellos.

## Estado en una tabla

| Medida | Valor | Cómo comprobarlo |
|---|---|---|
| Comandos respondidos por el binario | **34 de 65** | `rg -n "Comandos en Go" MIGRATION.md` |
| Comandos aún en shell | **31** | `rg -c "\| shell \|" MIGRATION.md` |
| Ficheros en `console/commands/` | **46** | `fd -e sh . console/commands \| wc -l` |
| Gemelos de shell retirados | **19** | `rg -A3 '^RETIRED=' tests/unit/command_safety_test.sh` |
| Portados que **aún conservan** su `.sh` | **15** | ver la pista A, y `composer` en la B |
| Tests Go / aserciones de shell | 489 / 620 | `go test -json ./...` (pruebas y subpruebas que pasan) · `./tests/run.sh unit` |

## Qué hacer después, en orden

1. **Pista A — retirar los gemelos de shell que ya tienen port en Go.** Es lo más barato y lo que
   más desbloquea. Empieza por los pequeños y ya probados.
2. **Pista B — portar los 31 comandos que siguen en shell, y el baile del vendor de `composer`.**
   `create-project` primero: es el único llamador de `exec.sh` que no está portado.
3. **Pista C — defectos conocidos.** Ninguno bloquea; varios llevan cinco ciclos aplazados.

## Pista A — gemelos de shell pendientes de retirar

Estos 14 comandos están tabulados como `go` en `MIGRATION.md`, pero su `.sh` sigue en el árbol.
`composer` también lo está, pero no es un gemelo: está en la pista B.
Antes de borrar cada uno hay que **confirmar que está probado en Go** (el segundo paso de la regla);
algunos no lo están.

| Comando | Líneas `.sh` | Notas |
|---|---|---|
| `restart` | 4 | |
| `docker-compose` | 20 | |
| `docker-stop-all` | 20 | **peligroso**: nunca ejecutarlo de forma que pueda confirmar |
| `exec` | 22 | último de su cadena: lo llaman `composer` y `create-project`, los dos de la pista B |
| `masquerade` | 22 | probado en la costura; ejecuta un anonimizador real |
| `web` | 23 | |
| `worktree` | 26 | |
| `stop` | 34 | |
| `start` | 61 | |
| `down` | 78 | |
| `mysql` | 122 | |
| `setup` | 144 | pendiente de pruebas Go (`setup/doctor/clean-go-tests`) |
| `doctor` | 185 | pendiente de pruebas Go |
| `clean` | 335 | pendiente de pruebas Go |

Orden sugerido: por tamaño, empezando por `restart`. Antes de cada uno, además de apartarlo, busca
quién lo llama directamente (`rg '"\$COMMANDS_DIR"/<nombre>\.sh' console`). `exec.sh` cae cuando
caigan `create-project.sh` y `composer.sh`, los dos de la pista B.

Medir la lista de nuevo:

```bash
for n in $(rg "\| go \|" MIGRATION.md | awk -F'|' '{gsub(/[ `]/,"",$2); print $2}'); do
  [ -f console/commands/$n.sh ] && echo "$n $(wc -l < console/commands/$n.sh)"
done
```

## Pista B — comandos aún en shell (31)

| Área | Comandos |
|---|---|
| environment | `launch`, `post-start`, `proxy`, `rebuild`, `share`, `tui`, `tunnel` |
| tools | `cloud`, `cloud-login`, `compatibility`, `config-env`, **`create-project`**, `ssl`, `verify` |
| database | `db`, `dbeaver`, `sequelace`, `tableplus`, `transfer-db` |
| magento | `grunt`, **`install`** |
| ai | `ai-context`, `ai-doctor`, `ai-init`, `ai-pull`, `ai-reset`, `mcp`, `permissions` |
| files / versions | `transfer-media`, `switch`, `update` |
| composer | **portar el baile del vendor de macOS**: `install`, `update`, `require` y `remove` siguen cayendo a `composer.sh` en darwin (`mirrorsVendor`, `internal/cli/php.go`; `internal/cli/run.go:67-72`). Es la implementación viva ahí, no un gemelo, y no se retira hasta portarlo |

Ciclos ya identificados: `create-project-in-go`, `install-and-ssl-in-go`, `hosts-config-in-go`, y
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
| Apartar un `.sh` no ve a los scripts que lo llaman directamente si ninguna suite los ejecuta, o si lo sustituyen por un stub | Pasó con `magento.sh`: lo llamaban `install.sh` y tres tareas, y apartarlo sólo rompió `worktree_test.sh:145`. Esas llamadas ya van por el binario; el punto ciego de la regla sigue | Antes de borrar, además de la suite: `rg '"\$COMMANDS_DIR"/<nombre>\.sh' console` |

Ya resueltos (no reabrir): CI sin macOS, `gofmt` sin `./dockergento`, CI corriendo `tests/unit`
como "todo" — los tres en `d03d54c`. El aviso de worktree «los contenedores sirven el código de…»,
que el binario no daba para ninguno de los ocho comandos de código que responde, en `f03a5e4`.

## Reglas que no se negocian

| Regla | Por qué |
|---|---|
| Commits en `release/2.0.0`, en inglés, Conventional Commits, **sin firma de IA** (ni `Co-Authored-By` ni `Claude-Session`) | Norma del proyecto |
| Push, PR y release los decide el humano | |
| Nunca ejecutar `hm debug-on`/`debug-off`, `masquerade`, `docker-stop-all` contra un proyecto real | Editan contenedores, reinician servicios o borran |
| Ningún test puede llegar al daemon real; hay proyectos ajenos levantados | Fija `DOCKER_HOST=unix:///nonexistent` cuando corras suites amplias |
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
