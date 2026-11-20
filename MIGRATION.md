# Migración a Go — dónde estamos

> **El objetivo, para que no se pierda entre tandas:** que no quede ningún comando en shell, que
> la CLI, la TUI y la web se alimenten de las mismas piezas, que no haya nada duplicado y que la
> arquitectura hexagonal se sostenga de verdad. El orden entre tandas se optimiza; el destino no
> se negocia. Lo único que seguirá siendo shell son los comandos que un proyecto añada en
> `config/hm/commands`, y a esos los ejecuta el propio binario (`internal/cli/project_command.go`).
> `bin/run` y el árbol `console/` ya no existen (ver «Cierre» abajo).
>
> El documento que se abre al empezar una sesión. Las decisiones están en
> [docs/research/2.0-arquitectura.md](docs/research/2.0-arquitectura.md); esto es el estado.
>
> **Regla de oro**: cada comando que se porta llega con los tests del shell portados, y con el
> mismo contrato — mismos códigos de salida, mismo `--json`, mismas preguntas.

## Estado

| | |
|---|---|
| Rama | `release/2.0.0` |
| Fase | **2 · esqueleto y puente**, terminada · **3 · Docker por SDK**, en marcha |
| Comandos en Go | 65 de 65 |
| El binario | `go build -o bin/hm ./cmd/hm` |
| La suite | `go test ./...` y `./tests/run.sh` |

## Cierre: el punto de entrada de shell, borrado

Con los 65 comandos, la ayuda, `--version` y los comandos del proyecto respondidos por el binario,
`bin/run` y todo `console/` se borraron en la rama `delete-bin-run`:

- `1094e18` — el binario deriva `COMPOSE_PROJECT_NAME` del directorio cuando el proyecto no lo
  configura (lo último que sólo hacía `bin/run`).
- `d564d01` — fuera `bin/run`, `console/` (salvo `console/hm-completion.bash`, que los perfiles de
  shell de los usuarios cargan por esa ruta), el paquete `dockergento/adapters/legacy`, la opción
  `ShellRoot`, la ruta `HM_LEGACY_ROOT` de `selfrun` y la negativa `command_moved_to_binary`.
  Un comando del proyecto ya no recibe `HELPERS_DIR`, `TASKS_DIR` ni `COMPONENTS_DIR`.
- `d67db23` — fuera las suites que sólo probaban el árbol de shell; las demás preguntan a `bin/hm`.
  `08dce8d` — el clon de `version_switching_test.sh` crea `bin/`, que ya no guarda nada versionado.
- El commit siguiente a ese — la documentación (README, CLAUDE.md, `architecture/`, este fichero,
  `ROADMAP.md` y los specs de `openspec/specs/`) describe el estado final.

Lo que el shell hacía y el binario no: convertir el antiguo `config/docker/properties` en
`properties.json` en cada invocación (`load_properties.sh`, `refactor_old_version`). Está en la
pista C de `ROADMAP.md`. El resto de este documento es historia y se conserva como tal.

## Cómo continuar si esta sesión se corta

```bash
git checkout release/2.0.0
go build -o bin/hm ./cmd/hm    # el binario
go test ./...                  # los tests de Go
./tests/run.sh unit            # la suite de bash, la parte rápida
./bin/hm hm-go-project         # lo que la capa Go resuelve aquí
```

**El andamio, y cuándo se cae.** Queda una entrada que no es comando —no salen en `--help` ni
están en `command_descriptions.json`, y llevan guion bajo porque es lo que ese fichero ya usa para
las claves que no son comandos—. Existen porque, mientras la disciplina sea paridad byte a byte,
un comando portado no puede reportar nada que el de bash no reporte, así que lo que sólo sabe la
capa Go no tiene dónde mirarse. Cada una muere en un sitio concreto:

| | qué enseña | desaparece cuando |
|---|---|---|
| `hm _registry` | lo que guarda el registro | nada en shell necesite leerlo — condición cumplida desde que `bin/run` no existe; sólo lo miran ya las suites (`go_passthrough_test.sh`) |

El código está en dos mitades. `dockergento/` es la herramienta como librería —dominio, puertos,
casos de uso, adaptadores y una fachada— y es pública porque `internal/` es una regla del lenguaje
que impide importarla desde otro módulo. `internal/cli/` es la terminal, y es lo único privado:
importa la fachada y los tipos del dominio, y nada más. Ahí es donde entrarán HTTP y MCP, como dos
puertas más sobre las mismas llamadas.

Lo siguiente por hacer está siempre en la primera fase sin terminar de la lista de abajo, y el
detalle de cada una en `openspec/changes/`.

## Las pruebas

Dos suites, y la balanza entre ellas se mueve.

Lo escrito en Go se prueba en Go: los paquetes de debajo, y en `test/e2e` el binario construido,
ejecutado como lo ejecuta una persona. Lo que sigue en shell se prueba en shell, y su suite se va
cuando se va el comando.

La regla, que es la misma disciplina que el resto del port: **un comando portado → su prueba en Go
→ se borran la implementación de shell y su prueba de bash**, en ese orden. Nada de shell se borra
antes de que su comportamiento esté cubierto en Go. Mientras las dos mitades existan, la prueba de
paridad se queda en bash, porque es lo que es: un diff entre un binario y un script, y la mitad de
sus entradas son funciones de bash.

`tests/run.sh` corre las dos y reporta las dos, para que una prueba que cambia de sitio no se lea
como cobertura que desaparece. Las de Go van primero, porque son segundos y no tiene sentido
enterarse de que fallan después de veinte minutos de Docker.

### Cómo se ejecuta el tercer paso

**Antes de borrar un gemelo de shell: apártalo, corre la suite entera, anota qué se rompe,
restáuralo, repara, y entonces bórralo.** Una comprobación textual vale para saber dónde mirar.
Nunca es prueba de ausencia.

Está escrito porque costó tres intentos aprenderlo, en la retirada de `list`, `describe` y `logs`:

1. **Sin comprobación.** Un ciclo anterior borró `version.sh` sin mirar nada. Cuatro aserciones
   fallaron durante un día, y dos pasaron por el motivo equivocado.
2. **Grep por nombre de comando.** Entre 21 y 34 ficheros, casi todos falsos positivos, porque
   `list` es además subcomando de `db`, `worktree`, `proxy` y `snapshot`.
3. **Grep resolviendo además la variable del punto de entrada.** Se escribió a mitad de ciclo como
   "la comprobación que funciona" y **también estaba mal**: enumeraba los wrappers que alguien ya
   conocía — `$HM`, `$SHELL_CLI`, `$GO_BINARY`, `run_in_lab`, `run_in`, `both` — y se dejó dos suites cuyo
   helper se llama simplemente `run`. Una enumeración de los wrappers que conoces no puede
   encontrar el que no conoces.
4. **Apartar los ficheros y correrlo todo.** Encontró las dos suites perdidas, y una tercera que
   ningún grep de invocaciones habría visto jamás: `tests/unit/tui_actions_test.sh` comprueba que
   cada comando ofrecido por el dashboard tenga su `.sh`, y no invoca nada.

Si la suite completa toca el daemon y no puedes dejarla, corre las dos veces —con los ficheros y
sin ellos— contra un `DOCKER_HOST` inalcanzable, y quédate con la **diferencia**: el ruido aparece
igual en ambas y se cancela. Restaura copiando el respaldo encima, nunca con `git checkout <fichero>`,
que en este repo ya revirtió una unidad entera sin commitear dejando `git status` limpio.

Y una lectura que conviene hacer cada vez: si borrar la implementación de shell no rompe **nada**,
es que nadie la estaba probando. El verde es ausencia de cobertura, no presencia de prueba.

### Cómo se escriben

Las convenciones son las de la comunidad de Go, no unas nuestras:

- **Tablas con subpruebas.** Un mapa de casos con nombre y un `t.Run` por caso. El nombre del caso
  es la clave, así que el fallo dice cuál falló sin leer el código, y `-run` ejecuta uno solo. El
  mapa además se recorre en orden aleatorio, lo que impide que un caso dependa del anterior.
- **`got` antes de `want`,** nombrando la entrada: `Slug("Feature/X") = "feature-x", want "feature-x"`.
  En inglés, como el resto del código.
- **`t.Errorf` por defecto y `t.Fatalf` sólo cuando lo que viene después no tiene sentido.** Una
  prueba que se para en el primer fallo esconde los otros tres.
- **`t.Helper()`** en los ayudantes, **`t.Cleanup`** en vez de `defer`, **`t.TempDir()`** para lo
  que se escribe. `t.Setenv` **no** se puede usar en pruebas paralelas: afecta a todo el proceso.
  Por eso las e2e pasan el entorno explícito en cada invocación en vez de exportarlo.
- **`cmp.Diff`** para comparar cosas compuestas, en vez de campo a campo.
- **Lo lento se salta con `-short`** además de saltarse solo si no hay demonio: `go test -short ./...`
  es la respuesta rápida en cualquier máquina.
- Sin librerías de aserciones: se compara con Go, que es lo que recomienda la guía de estilo de
  Google y lo que hace que un fallo diga algo.
- **Contra `newEngine`, no contra Docker.** Lo que pide un comando ya cableado en Go se prueba
  sustituyendo esa fábrica por un doble que registra las llamadas: sin demonio, sin red y sin
  ningún proyecto real de por medio.

## Fases

- [x] **0 · Estabilizar la 1.x.** Concurrencia, colisiones, `vendor` montado, cachés que no
  invalidaban, chequeo de memoria de la VM. Prueba de diez worktrees pasada.
- [x] **2 · Esqueleto y puente**
  - [x] Módulo, dominio, puertos y casos de uso separados de los adaptadores
  - [x] Puente a bash: todo lo no portado corre igual, con sus códigos de salida
  - [x] Resolución del proyecto en Go (raíz, worktree, properties), probada contra la de bash
  - [x] CI que compila, formatea, pasa `vet`, los tests de Go y la suite unitaria de bash
  - [x] Distribución: `goreleaser` y binarios para darwin/linux, amd64/arm64
  - [x] Instalación: `hm` es el binario, con vuelta al shell si no se puede descargar
- [x] **3 · Adaptador de Docker (SDK) + tanda 1** — donde gana todo el equipo
  - [x] Adaptador del demonio por SDK, resolviendo el socket desde el contexto de Docker
  - [x] `list` — el primero: sólo lectura, puro Docker, salida idéntica byte a byte
  - [x] `describe` — el más usado y el de contrato más rico
  - [x] `doctor` — diecisiete comprobaciones, cinco a Docker y cuatro a la máquina
  - [x] `start`, `stop`, `restart`, `logs`, `exec` — Compose como librería (ADR-009 bis)
  - [x] `magento`, `composer` — el baile del vendor en macOS sigue en shell
- [x] **4 · Registro SQLite con las dos topologías + tanda 2**
  - [x] Esquema, slots atómicos e importación de lo que escribió bash
  - [x] `mysql` entero, import incluido — y con él consultar, alimentar un contenedor, lanzar uno
        suelto, el indicador de progreso y preguntar por terminal, que necesita el resto de la tanda
  - [x] `worktree` entero — `add`, `list` y `remove`. La registración y el overlay que escribe son
        byte a byte los de bash, comprobado
  - [x] el registro es la fuente viva. `console/commands/worktree.sh` pasó a delegar en el binario,
        así que bash ya no escribe registraciones y el adaptador pudo cambiar de golpe. Lo que
        escribió la 1.x se arrastra al leer y se borra al olvidar el worktree: una máquina con
        ramas montadas las conserva, y nada queda diciendo algo que ya no es verdad. El overlay
        sigue siendo un fichero, porque un compose en una base de datos no lo carga nadie. La
        mitad de bash no puede abrir la base de datos, así que el binario le entrega la
        registración al puentear, y el punto de entrada de shell lanzado a solas se la pide: un
        comando puenteado desde una rama que no encontrase registración resolvería al entorno
        principal, que es justo lo que WT-01 existe para impedir
  - [x] `db` entero — las plantillas (`freeze`, `templates`, `clone`, `drop`), de las que depende
        `worktree`, y las instantáneas (`snapshot`, `list`, `restore`, `remove`, `clear`), de las
        que depende `down`. El texto de uso y la negativa a un subcomando desconocido también
        los contesta ya el binario, y sin pedir Docker: `bin/run` lo validaba antes de llegar a
        `db.sh`, igual que hacía con los errores de argumentos de las dos familias
  - [x] `clean` — y con él ya nadie más escribe registraciones desde bash
  - [x] `down` — y con él el selector: la lista por la que se mueve uno, con `fzf` si está
        instalado, con las flechas si se puede dibujar, y numerada donde no. Es la pieza que
        necesitan también `setup` y los asistentes, y ahora `stop --snapshot` toma la copia
        directamente en vez de pasar por el shell para hacerlo
  - [x] `proxy` — arriba, abajo y estado, con el mismo fichero de compose byte a byte y las rutas
        leídas de la API del propio Traefik. Con él, arrancar el proxy para un proyecto que lo
        necesita deja de pasar por el shell
  - [x] `setup` — las preguntas, las propiedades y los ficheros de compose que genera, incluido el
        overlay del proxy byte a byte. La instalación que viene después sigue siendo del shell,
        comando a comando: tres de ellos aún no están portados
- [ ] **5 · Servicios compartidos, seed, worktrees, GC** — donde gana el trabajo con agentes
- [ ] **6 · Adaptadores de agente: `--json`, MCP, HTTP para la web** — *empezada*
  - [x] `hm web`: la API HTTP y la interfaz de navegador, sobre las mismas llamadas que la CLI
  - [ ] MCP en Go
- [ ] **7 · Tanda 3 y tanda 4: hasta que no quede ningún comando en shell** — *empezada*
  - [x] Los envoltorios: `purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`,
        `mysqldump`, `varnish-on` y `varnish-off`. Todos son una cosa ejecutada en un contenedor, y
        lo que no podían era diferir entre sí en cómo llegan a él
  - [x] `version` — y con él muere `hm _binary`: qué build del binario corre es un campo suyo
  - [x] `copy-to-container` y `copy-from-container` — por SDK, con su propio tar, que es lo que
        hace `docker cp` sin el proceso. Un camino que ya es un bind mount se rechaza en vez de
        copiarse encima de sí mismo
  - [x] `set-host` — con el fichero de hosts redirigible, así que por primera vez está probado lo
        que le hace: que no añade nada si el nombre ya resuelve aquí, y que sólo borra lo que puso
        él
  - [x] El tercer paso, para estos doce: su gemelo de shell y la prueba de bash que lo comparaba
        ya no existen. `set-host.sh`, `copy-to-container.sh`, `copy-from-container.sh` y los nueve
        envoltorios (`version`, `varnish-on`, `varnish-off`, `mysqldump`, `purge`, `n98-magerun`,
        `test-unit`, `npm`, `test-integration`) están borrados de `console/commands/`, con la misma
        disciplina de las líneas 62-66: nada de shell se borra antes de que su comportamiento esté
        cubierto en Go. Las seis pruebas de paridad de `tests/integration/go_wrappers_test.sh` que
        comparaban un binario con el otro se reescribieron contra un oráculo que no es ninguno de
        los dos — un contenedor grabando su propio argv, la base de datos restaurada y contada, y
        git en el directorio de instalación —, y `tests/unit/command_safety_test.sh` ganó la
        dirección que faltaba: que lo que el binario ya posee del todo no tiene fichero del que
        desviarse

        La afirmación de arriba — "nada de shell se borra antes de que su comportamiento esté
        cubierto en Go" — se cumplió para once de los doce gemelos, no para `varnish-on`. Su gemelo
        de shell, `console/commands/varnish-on.sh` (visible en `git show 583fb0c^:...`), abría con
        `set -euo pipefail`: un `sed` que fallaba terminaba el script ahí mismo, con el código de
        salida del propio `sed`, sin reintentar el `restart` y sin mensaje alguno. Ese
        comportamiento no estaba cubierto en Go hasta `wrapper-exit-codes`, que lo cierra para
        `varnish-on` y `varnish-off`: antes, un `sed` o un `cache:enable`/`cache:disable` que
        fallaban sin error de Go se reportaban como éxito.
  - [x] El mismo paso, para `list`, `describe` y `logs` — decimotercero, decimocuarto y
        decimoquinto comandos retirados así. `list.sh`, `describe.sh` y `logs.sh` (223 líneas)
        están borrados de `console/commands/`: las tres tienen su propia prueba de interacción con
        el motor, y todo lo que las invocaba a través de `bin/run` se corrigió antes de borrarlas.
        Las dos comparaciones de paridad que les quedaban — `tests/integration/
        go_passthrough_test.sh` (los bloques de `describe` y `list`) y `tests/integration/
        go_orchestration_test.sh` (el bloque de `logs`) — se reescribieron contra un oráculo que no
        es ninguna de las dos implementaciones, en vez de comparar el documento entero o la tabla
        renderizada byte a byte, que es lo que se pierde y queda dicho, no disimulado. Y
        `tests/unit/command_safety_test.sh` ganó las tres en su lista `RETIRED`, en el mismo commit
        que borra los ficheros, para que la comprobación en ambas direcciones — fichero fuera,
        ruta dentro — también las cubra.
  - [x] El mismo paso, para `debug-on` — decimosexto comando retirado así. `console/commands/
        debug-on.sh` (38 líneas) y `console/tasks/set_xdebug_host_property.sh` (19) están borrados;
        quedan 49 ficheros en `console/commands/`. La tarea se va sin mecanismo propio: sólo la
        cargaba `debug-on.sh` con `source`, y las tareas no se despachan, así que desaparece como
        efecto del borrado del comando.

        **Esta vez no hubo nada que reparar, y eso dice algo.** Apartar los dos ficheros y correr
        la suite entera no cambió nada: 59 de 1003 aserciones fallaban antes y después, en las
        mismas siete suites, con el diff entre las dos corridas vacío. Ninguna aserción de shell
        nombraba este comando. No se retira ninguna comparación de paridad porque **nunca existió
        ninguna**: `wrappers_test.go` prueba el handler de Go contra un fake, jamás contra la
        salida real del script. Si borrar la implementación entera no rompe nada, es que nadie la
        estaba probando — el verde es ausencia de cobertura, no presencia de prueba.

        **Doce comentarios quedan citando un fichero que ya no está**, y es decisión, no descuido:
        `internal/cli/wrappers.go` (`:376`, `:381`, `:468`, `:472`, `:474`, `:496`, `:519`),
        `wrappers_test.go` (`:1476`, `:1484`) y `openspec/specs/go-entrypoint/spec.md` (`:466`,
        `:570`, `:606`) citan `debug-on.sh:NN` como procedencia de un comportamiento portado. Siguen
        siendo ciertos: afirman lo que el shell hacía, y borrarlo no lo desmiente. Los números de
        línea son justo lo que los hace comprobables con `git show`. El coste, dicho y no escondido:
        el primer instinto de quien los lea es abrir el fichero, y eso ahora falla doce veces —
        `git show 8d0b74a^:console/commands/debug-on.sh` es donde está.

        `openspec/specs/go-entrypoint/spec.md:606` conserva además una premisa que se queda vacía
        — «GIVEN nothing in this repository sets `XDEBUG_HOST` except `set_xdebug_host_property.sh`
        itself» —, porque vive dentro de un requisito de unas 600 líneas que cubre `debug-off`,
        `magento` y `logs`, y el flujo de MODIFIED obligaría a copiarlo entero para arreglar una
        cláusula cuya conclusión el borrado sólo refuerza.
  - [x] El mismo paso, para `debug-off` — decimoséptimo comando retirado así.
        `console/commands/debug-off.sh` (9 líneas) está borrado; quedan 48 ficheros en
        `console/commands/`. Nada en el repositorio lo nombraba: ni una suite, ni el dashboard, ni
        otro script. Como con `debug-on`, apartarlo y correr la suite entera no cambió nada — 59 de
        1006 aserciones fallaban antes y después, las mismas siete suites, diff vacío entre las dos
        corridas contra un `DOCKER_HOST` inalcanzable. Tampoco aquí se retira comparación de
        paridad alguna, porque nunca la hubo.

        Con él se va **uno de los cinco llamadores de `exec.sh`**. Quedan cuatro: `magento.sh`,
        `bash.sh`, `create-project.sh` y `composer.sh`. `exec.sh` no puede retirarse hasta que se
        vayan los cuatro. (Corregido después: aquí decía que ninguno estaba portado, y no era
        verdad. `magento` y `bash` ya los respondía el binario; `composer` también, salvo
        `install`, `update`, `require` y `remove` en macOS, que siguen cayendo a `composer.sh`
        — `mirrorsVendor`, `internal/cli/php.go` — porque el baile del vendor sigue en shell. El
        único sin port es `create-project`.)

        El script llevaba un escape muerto que el port ya había dejado fuera:
        `s/^\zend_extension/;zend_extension/g`, donde `\z` no significa nada en una expresión
        regular de `sed` y sólo funcionaba porque se reduce a la `z` literal. Queda dicho aquí
        porque es el mismo hallazgo que `varnish-on` dio en su día, y es el tipo de detalle que un
        port transcrito arrastraría sin mirarlo.
  - [x] El mismo paso, para `bash` — decimoctavo comando retirado así. `console/commands/bash.sh`
        (8 líneas) está borrado; quedan 47 ficheros en `console/commands/`. `TestWhatBashAsks`
        (`internal/cli/tools_test.go`) ya cubría lo que hacía: `bash` sin argumentos, `-r` como
        usuario root, y que cualquier otro argumento se descarta igual que lo descartaba el script.
        Apartarlo y correr la suite entera no cambió nada — 59 de 1017 aserciones fallaban antes y
        después, las mismas siete suites, diff vacío entre las dos corridas contra un `DOCKER_HOST`
        inalcanzable. Ningún script lo llamaba y el dashboard no lo ofrece; no hubo nada que
        reparar ni comparación de paridad que retirar.

        Con él se va **otro llamador de `exec.sh`**. Quedan tres: `magento.sh`, `create-project.sh`
        y `composer.sh`.

        Cuatro textos siguen nombrando el script, y se quedan: `tests/unit/skills_test.sh` (`:6`,
        `:126`), `docs/skills.md:14`, `docs/research/backlog.md:664` y `changelogs/v1.7.0.md:166`
        explican con él por qué `hm bash` no recibe nunca un comando. Lo que dicen sigue siendo
        verdad del port, que descarta igual lo que no sea `-r`. El fichero está en
        `git show fde5549:console/commands/bash.sh`.
  - [x] El mismo paso, para `magento` — decimonoveno comando retirado así.
        `console/commands/magento.sh` (3 líneas) está borrado; quedan 46 ficheros en
        `console/commands/`. `TestWhatMagentoAsks` y sus vecinos (`internal/cli/magento_test.go`)
        lo prueban contra el fake: `php ./bin/magento` en `phpfpm` con los argumentos tal cual, el
        código de salida del propio Magento sin traducir y nada ejecutado fuera de un proyecto.

        **Éste no era un gemelo puro, y apartarlo no lo dijo.** `install.sh` (`:119-120`) y tres
        tareas — `magento_installation.sh`, `set_magento_configs.sh` y `admin_bootstrap.sh` —
        llamaban a `"$COMMANDS_DIR"/magento.sh` directamente, sin pasar por `bin/run`: siete
        llamadas que el borrado habría roto en `hm install`, `hm setup`, `hm create-project` y
        `hm config-env`. La suite entera no vio ninguna, porque ninguna suite ejecuta esos scripts
        sin Docker y las dos de `admin_bootstrap` sustituían `magento.sh` por un stub. Las encontró
        un grep. Ahora van por `hm_run_binary magento`, y las suites de `admin_bootstrap` falsean el
        binario en vez del script, así que comprueban la llamada y no sólo su resultado. Apartar el
        fichero sigue siendo la regla, pero no basta: antes de borrar, también
        `rg '"\$COMMANDS_DIR"/<nombre>\.sh' console`.

        Lo único que la suite sí vio fue `tests/integration/worktree_test.sh` («a command touching
        code warns about whose code it is»), y **destapó un hueco del binario**: el aviso de que,
        desde un worktree sin entorno propio, los contenedores sirven el código del checkout
        principal sólo lo daba `bin/run`. Los ocho comandos de esa lista que ya responde el binario
        — `magento`, `composer`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `exec` y
        `bash` — lo habían perdido al portarse, y el único que lo seguía comprobando era el gemelo.
        El binario lo da ahora (`codeProjectOr`, `internal/cli/engine.go`), por stderr como el
        shell, y la prueba de `worktree_test.sh` le pregunta al binario.

        Con la repetición del paso — con el aviso portado y la prueba apuntando al binario —,
        apartarlo y correr la suite entera no cambió nada: 59 de 1025 aserciones fallaban antes y
        después, las mismas siete suites, diff vacío contra un `DOCKER_HOST` inalcanzable.

        Con él se va **otro llamador de `exec.sh`**. Quedan dos: `composer.sh`, que en macOS sigue
        siendo la implementación viva del baile del vendor, y `create-project.sh`, sin portar.
  - [x] El mismo paso, para `docker-compose` — vigésimo comando retirado así.
        `console/commands/docker-compose.sh` (20 líneas) está borrado; quedan 45 ficheros en
        `console/commands/`. No era más que un `exec` del binario: lo único que hacía por su cuenta
        era negarse con `binary_missing` si el binario no estaba, y eso desaparece con él —
        `bin/run docker-compose` contesta ahora `command_moved_to_binary`, como los otros
        diecinueve. Ninguna prueba dependía de ese mensaje suyo (`tests/unit/exit_codes_test.sh`
        prueba el de `hm_run_binary`, que es otro). `docker_compose_test.go` ya cubría los
        argumentos tal cual, el código de salida de Compose sin traducir (17), la negativa fuera de
        un proyecto y la falta de Compose.

        **Apartarlo no cambió nada, y esta vez eso no bastaba.** Ningún script lo llamaba
        directamente, pero dos suites le preguntaban a `bin/run docker-compose`, y ninguna de las
        dos lo dice sin un daemon: las dos comparaciones de
        `tests/integration/go_passthrough_test.sh` ya fallaban antes (`bin/run` valida Docker antes
        de nada), y `tests/integration/worktree_environments_test.sh` se salta entera. Las dos
        comparaciones eran además del binario consigo mismo, porque el gemelo sólo lo ejecutaba; se
        reescribieron contra un oráculo que no es ninguno de los dos — el proyecto que escribe el
        laboratorio y el propio `docker compose` —, y ahora pasan también sin daemon. La de
        `worktree_environments` le pregunta al binario. Con esas reparaciones, apartarlo y correr la
        suite entera: 57 de 1030 aserciones fallaban antes y después, las mismas siete suites, diff
        vacío.

        `restart` iba antes en la lista y **sigue en el árbol**: sus llamadores directos — el baile
        del vendor de `composer.sh` (`:93`), `create-project.sh` (`:69`, `:74`) y
        `magento_installation.sh` (`:29`) — no pasan por la protección de worktree de `bin/run`, y
        el `restart` del binario sí la aplica: desde un worktree sin entorno propio se niega con
        `blocked_in_worktree` (6). Mandarlos al binario cambiaría `hm composer install` en macOS
        desde un worktree así, que hoy reinicia el `phpfpm` del checkout principal y a partir de
        entonces se negaría. Es una decisión, no una reparación, y queda pendiente. Lo que sí queda
        hecho es la costura: el `restart` de Go pedía la operación al motor concreto, así que
        ninguna prueba podía decir qué pedía; ahora la pide por `newEngine` y `restart_test.go` lo
        fija.
  - [x] El mismo paso, para `restart` — vigésimo primer comando retirado así.
        `console/commands/restart.sh` (4 líneas) está borrado; quedan 44 ficheros en
        `console/commands/`. Era `stop.sh` y luego `start.sh`, y el binario hace lo mismo:
        `Operator.Restart` para y arranca, y en Linux devuelve `post-start` al shell como
        `start.sh`. `restart_test.go` fija lo que pide el handler y `orchestrate_test.go` el orden y
        la protección de worktree.

        Cuatro llamadas lo ejecutaban directamente, sin pasar por `bin/run`: `composer.sh:93`,
        `create-project.sh:69` y `:74`, y `magento_installation.sh:29`. Ahora van por
        `hm_run_binary restart`. **Eso cambia una cosa, aceptada a sabiendas:** en macOS,
        `hm composer install|update|require|remove` desde un worktree sin entorno propio se niega
        ahora con 6 (`blocked_in_worktree`), antes de reiniciar nada ni copiar el vendor. Antes
        reiniciaba el `phpfpm` del checkout principal y sincronizaba el vendor a través de él,
        porque la llamada directa se saltaba la protección de worktree que `bin/run` aplica a
        `restart`. La negativa la da `composer.sh` antes del reinicio, con su propia salida:
        `hm --force composer install`. La del binario decía «repeat with --force», y detrás de
        `composer` esa opción es de Composer, así que `hm composer install --force` nunca llegaba
        a `hm`; delante sí, se exporta como `HM_FORCE` y el `restart` de dentro la lee. Se queda
        así hasta que el baile del vendor esté portado. Las otras tres sólo se alcanzan desde
        `setup` y `create-project`, que ya se niegan en un worktree así.
        `tests/unit/composer_vendor_test.sh` lo fija sin Docker.

        Apartarlo antes de las reparaciones rompió una aserción, `worktree_test.sh` «restart is
        refused from a worktree», que le preguntaba a `bin/run`. Otra suite,
        `go_orchestration_test.sh`, comparaba `bin/run restart` con el binario desde un worktree y
        no lo dijo porque sin daemon se salta entera. Las dos le preguntan ahora al binario y
        comprueban su negativa contra el contrato — 6 y `blocked_in_worktree` —; con el binario
        compilado para forzar siempre, la de `worktree_test.sh` falla. Con las reparaciones,
        apartarlo y correr la suite entera: 57 de 1038 aserciones fallaban antes y después, las
        mismas siete suites, diff vacío.

        Cuatro sitios siguen citando `restart.sh` como procedencia —
        `internal/cli/wrappers.go:345`, `wrappers_test.go:822` y
        `openspec/specs/go-entrypoint/spec.md` (`:465`, `:570`) — y se quedan, como los de
        `debug-on`: dicen lo que el shell hacía.
  - [x] El mismo paso, para `docker-stop-all` — vigésimo segundo comando retirado así.
        `console/commands/docker-stop-all.sh` (20 líneas, sólo hacía `exec` del binario) está
        borrado; quedan 43 ficheros en `console/commands/`. `docker_stop_all_test.go` fija lo que
        pide el handler (`Interactive` según `HM_NON_INTERACTIVE`) y `orchestrate_test.go` la parada
        y la protección de worktree.

        Un llamador directo que ninguna suite ejecuta: `start.sh -s`. Ahora va por
        `hm_run_binary docker-stop-all`; el comportamiento es el mismo, porque el `.sh` ya era un
        `exec` del binario, y `start` desde un worktree sin entorno propio ya lo niega `bin/run`.

        Apartarlo antes de las reparaciones rompió cuatro aserciones en dos suites, las dos por
        preguntarle a `bin/run docker-stop-all`: `worktree_test.sh` «docker-stop-all is refused
        from a worktree» y `go_passthrough_test.sh` («the shell entry point still answers…» y «the
        question is asked on the caller's own terminal»). La primera le pregunta ahora al binario y
        comprueba 6 y `blocked_in_worktree`; con el binario compilado para forzar siempre, falla.
        **La segunda era peligrosa:** ejecutaba `docker-stop-all` con `HM_NON_INTERACTIVE=1`, que
        para todos los contenedores en marcha sin preguntar, y la suite no se salta con el daemon
        accesible: un `./tests/run.sh` corriente paraba todo lo que la máquina tuviera levantado.
        Ahora le pregunta al binario con `DOCKER_HOST` apuntando a un socket inexistente, dentro de
        la propia prueba, y comprueba que no hay nada que parar. La pregunta en la terminal del
        llamador era cosa del stub de delegación y la fija Go. Con las reparaciones, apartarlo y
        correr la suite entera: 57 de 1047 aserciones fallaban antes y después, diff vacío.
  - [x] El mismo paso, para `masquerade` — vigésimo tercer comando retirado así.
        `console/commands/masquerade.sh` (22 líneas) está borrado; quedan 42 ficheros en
        `console/commands/`. `tools_test.go` (`TestWhatMasqueradeAsks`) fija lo que pide el handler
        y que una opción desconocida no pide nada; `go_orchestration_test.sh`, que se salta sin
        daemon, que un «no» deja los datos en paz. El port conserva la confirmación, el «sí» sin
        terminal y el registro de la anonimización; declinar sale con 0 en vez del 1 accidental del
        shell. Nadie llamaba al `.sh`: `mysql.sh` y `transfer-db.sh` cargan el componente
        `console/components/masquerade.sh`, que se queda.

        Apartarlo no rompió nada: 57 de 1050 aserciones fallaban antes y después, diff vacío. Ese
        verde escondía un punto ciego: `non_interactive_test.sh` comprobaba que
        `bin/run masquerade` no se queda esperando, y retirado, `bin/run` se niega al instante y
        la comprobación pasa sin probar nada. Ahora se la hace al binario, con `DOCKER_HOST`
        apuntando a un socket inexistente dentro de la propia comprobación: sin nadie que
        conteste, `masquerade` toma el sí por respuesta. Y como medir sólo que vuelve lo aprobaría
        cualquier negativa, también comprueba cuál: 4 y `project_not_configured`, antes de preguntar.
  - [x] El mismo paso, para `web` — vigésimo cuarto comando retirado así.
        `console/commands/web.sh` (23 líneas, sólo hacía `exec` del binario) está borrado; quedan
        41 ficheros en `console/commands/`. No tenía ninguna prueba Go: la única era
        `go_web_test.sh`, que se salta sin daemon. Antes de borrarlo, `internal/cli/web_test.go`
        fija los argumentos, `status` contra un fichero de estado ausente, caducado o vivo,
        `down` contra uno caducado (nunca contra un pid vivo: le mandaría SIGTERM), que `up` encuentra el servidor que ya corre en vez de arrancar otro, y que un puerto
        ocupado se niega antes de escribir estado. `HOME` es temporal en cada prueba y ninguna
        arranca ni señala procesos; cuatro mutaciones, cada una compilando, rompen la suya.

        Nadie lo llamaba ni le preguntaba a `bin/run web`. Apartarlo: 57 de 1055 aserciones
        fallaban antes y después, diff vacío.
  - [x] El mismo paso, para `worktree` — vigésimo quinto comando retirado así.
        `console/commands/worktree.sh` (26 líneas, sólo hacía `exec` del binario) está borrado;
        quedan 40 ficheros en `console/commands/`. **Escondía un bucle:** `run.go` sólo enrutaba
        `add`, `list` y `remove`; `hm worktree` sin subcomando, o con uno desconocido, caía a
        `bin/run`, cuyo `worktree.sh` volvía a ejecutar el binario con los mismos argumentos. Con
        Docker levantado (la validación de `bin/run` pasa), binario → `bin/run` → `worktree.sh` →
        binario, anidando procesos hasta matarlos: con un `docker` falso que siempre responde 0,
        doce procesos en cuatro segundos. Ahora el binario responde a todo `worktree`, y un
        subcomando ausente o desconocido es un error de uso (2, `invalid_argument`, pista
        `hm worktree add|list|remove`); `worktree_usage_test.go` lo fija a través de `Run`, con el
        puente apuntando a un directorio sin `bin/run`.

        Las suites que le preguntaban a `bin/run worktree` le preguntan al binario:
        `worktree_environments_test.sh` y `concurrent_safety_test.sh`, que se saltan o fallan sin
        daemon y no lo habrían dicho, y el experimento `ten_worktrees.sh`. Apartarlo antes de
        repararlas cambió una aserción en el sentido contrario al habitual:
        `go_passthrough_test.sh` «a usage error keeps its code» comparaba `bin/run worktree add`
        con el binario; con el `.sh` era la comprobación de Docker de `bin/run` (3) contra el error
        de uso del binario (2), y sin él la negativa de comando retirado, que también sale con 2,
        por casualidad. Ahora sólo le pregunta al binario. Con las reparaciones, apartarlo y correr
        la suite entera: 54 de 1057 aserciones fallaban antes y después, diff vacío.
  - [x] El mismo paso, para `stop` — vigésimo sexto comando retirado así.
        `console/commands/stop.sh` (34 líneas) está borrado; quedan 39 ficheros en
        `console/commands/`. El handler de Go le pedía la parada al motor concreto, como `restart`,
        así que ninguna prueba podía decir qué pedía; ahora va por `newEngine` y `stop_test.go`
        fija los servicios y `--snapshot` que pasa, que fuera de un proyecto no pide nada y que una
        negativa de worktree conserva su código y su tipo. Lo que es una parada — la copia antes
        y nada parado si falla, la protección de worktree — es de `app.Operator`.

        Dos llamadores directos que la suite no ejecuta: `rebuild.sh` y el baile del vendor de
        `composer.sh` en macOS. Van por `hm_run_binary stop`; `rebuild` ya lo niega `bin/run`
        desde un worktree y el baile se niega antes, así que la protección del binario no añade
        ninguna negativa. Cuatro suites le preguntaban a `bin/run stop` y le preguntan al binario
        (`worktree_test.sh`, `tui_dashboard_test.sh`, `lifecycle_guard_test.sh`,
        `go_orchestration_test.sh`); varias se saltan o fallan sin daemon y no lo habrían dicho.
        Apartarlo, ya reparadas: 54 de 1061 aserciones fallaban antes y después, diff vacío.

        Una diferencia con el shell, heredada del port y no de esta retirada: `stop.sh` pasaba a
        `docker compose stop` cualquier opción (`hm stop -t 5`); el binario la toma por un
        servicio. Queda en la pista C.
  - [x] El mismo paso, para `start` — vigésimo séptimo comando retirado así.
        `console/commands/start.sh` (61 líneas) está borrado; quedan 38 ficheros en
        `console/commands/`. El `Start` de Go ya hacía todo lo que hacía el `.sh` — protección de
        worktree, `-s`, el proxy, `up`, la comprobación de bind mounts y `post-start` en Linux —
        pero el handler pedía al motor concreto. Ahora va por `newEngine` y `start_test.go` fija
        los servicios y `-s` que pasa, si parar el resto puede preguntar, que cualquier otra opción
        es un error de uso antes de resolver nada y que una negativa conserva su código.

        Un llamador directo: el baile del vendor de `composer.sh`, que va por
        `hm_run_binary start`. Las suites que le preguntaban a `bin/run start` le preguntan al
        binario. `go_orchestration_test.sh`, que se salta sin daemon, comparaba además con gemelos
        **ya retirados** — `bin/run magento` y `bin/run bash`, desde `cd0afc8` y `65ecb65` — y con
        daemon esas aserciones fallaban sin que nadie lo viera. Ahora `magento` y `bash` se
        comparan con el `exec` que ejecutaban, el hash y las etiquetas de Compose con el
        `docker-compose` del binario (lo que `start.sh` ejecutaba por dentro), y las negativas
        desde un worktree de `restart`, `start` y `stop` con el contrato. `non_interactive_test.sh`
        le pregunta al binario por `bash` y `start`, que `bin/run` negaría al instante.

        Apartarlo rompió una comprobación textual: `post_start_test.sh` leía `start.sh` para ver
        que llamaba a `post-start.sh`. La entrega del binario la fija Go
        (`TestLinuxIsHandedBackWhatIsNotPortedYet`). Reemplazarla destapó que `rebuild.sh` lleva su
        propia copia de esos pasos: pista C. Con las reparaciones, apartarlo: 54 fallos antes y
        después, diff vacío.
  - [x] El mismo paso, para `down` — vigésimo octavo comando retirado así.
        `console/commands/down.sh` (78 líneas) está borrado; quedan 37 ficheros en
        `console/commands/`. El `Down` de Go ya hacía lo del `.sh` — la pregunta de tres
        respuestas antes de borrar volúmenes, la copia primero y nada destruido si falla, la
        destrucción sin preguntar con `--yes` — pero el handler pedía al motor concreto. Ahora va
        por `newEngine` y `down_engine_test.go` fija las opciones que pasa, si la pregunta puede
        hacerse, el mensaje cuando no se destruye nada, el informe `--json` de volúmenes y copia, y
        que una negativa conserva su código. `down_test.go` ya fijaba los rechazos al leer.

        Nadie lo llamaba directamente. Las suites que le preguntaban a `bin/run down` le preguntan
        al binario (`worktree_test.sh`, `project_name_test.sh`, `lifecycle_guard_test.sh`,
        `db_snapshots_test.sh`), y `go_orchestration_test.sh` comprueba su negativa desde un
        worktree contra el contrato. `lifecycle_guard_test.sh` contesta a la pregunta con un
        número por stdin, que el `choose` del binario acepta igual que `custom_select`.

        La primera corrida con el fichero apartado no valió: el disco se quedó sin espacio y dos
        suites no pudieron compilar el binario y se saltaron. Repetida con espacio: 54 de 1072
        aserciones fallaban antes y después, diff vacío.

        Revisado (`review-f1850f232ab7d6f2`). Queda una debilidad aceptada: en
        `go_orchestration_test.sh`, `magento` y `bash` se comparan con el `exec` del mismo binario,
        que es por donde pasan; el comando exacto que construyen lo fija `magento_test.go` en la
        costura, no esa comparación.
  - [x] El mismo paso, para `mysql` — vigésimo noveno comando retirado así.
        `console/commands/mysql.sh` (122 líneas) está borrado; quedan 36 ficheros en
        `console/commands/`. El handler de Go ya leía `-q`, `-i`, `-d` y `-a` y el `Import` del
        motor hacía lo mismo que el `.sh` — conservar el dominio, olvidar la anonimización
        anterior, anonimizar si se pide y dejar la tienda en desarrollo —, pero pedía al motor
        concreto. Ahora va por `newEngine` y `mysql_test.go` fija la consulta con su estado y su
        salida, las opciones de importación, la tilde, que un fichero que no existe no importa
        nada, el volcado por stdin y que una invocación mal formada no pide nada. La sesión
        interactiva no: necesita un terminal en stdin y `go test` nunca lo tiene.

        Tres llamadores directos que la suite no ejecuta: `set_etc_hosts.sh` y los dos de
        `magento_installation.sh` (uno importa por stdin). Van por `hm_run_binary mysql`. Un
        cuarto, `get_domains.sh`, llamaba a `mysql.sh` **sin** `-q` y no lo llamaba nadie; está
        borrado, y `architecture/05` dejó de atribuírselo a `ssl` y `set-host`.
        `go_database_test.sh`, que necesita daemon, compara ahora el binario con el cliente dentro
        del contenedor `db` — lo que ejecutaba `mysql.sh` — en vez de con `bin/run`;
        `global_options_test.sh` dejó de leer el `getopts` de `mysql.sh`, cuyo orden `-d -i` fija
        Go. Apartarlo: 54 de 1073 aserciones fallaban antes y después, diff vacío.

        Buscando otros pasos delegados al shell salió una regresión de la retirada de `magento`:
        `set-host` escribía la URL base con `magento config:set` a través de `bin/run`, que desde
        `cd0afc8` sólo nombra el binario, y `hm set-host <dominio>` sin `--no-database` fallaba con
        `config_failed`. Ahora va por el propio binario, como los pasos de `setup` (`416f72b`).
  - [x] El mismo paso, para `setup` — trigésimo comando retirado así.
        `console/commands/setup.sh` (144 líneas) está borrado, con las dos tareas que sólo él
        cargaba (`setup_options.sh`, `start_service_if_not_running.sh`); quedan 35 ficheros en
        `console/commands/`. El handler pedía al motor concreto; ahora va por `newEngine` y
        `setup_test.go` fija que la instrucción entera llega a un `Setup`, si puede preguntar, que
        una mal formada no crea nada, el informe de dónde responde el proyecto y que un fallo sale
        con su código. `setup_options_test.sh` probaba reglas que en Go no fijaba nadie: cada
        opción en sus formas larga, con espacio y corta, que nada dado no decide nada, que un
        argumento suelto se niega, que un volcado inexistente para el comando antes de nada y que
        `~` se expande. Están en `core/setup_parse_test.go` y `app/setup_dump_test.go`, y la prueba
        borrada se lleva sus 33 aserciones; la de que las opciones están declaradas en
        `command_descriptions.json` sigue, en `setup_declared_options_test.sh`.

        **Faltaba una protección:** `bin/run` negaba `setup` desde un worktree sin entorno propio;
        el binario no, y desde uno así reescribía el `config/docker/properties.json` versionado con
        el nombre de otro proyecto antes de fallar por cualquier otra cosa. Se vio porque
        `worktree_test.sh` se quedó esperando una respuesta en vez de recibir el 6. Ahora
        `Engine.Setup` aplica la misma negativa que `start`, `stop` y `down`, compartida con
        `Operator`, y sólo `--force` la levanta (`2005a93`). Cuando el proyecto no se resuelve —
        propiedades versionadas rotas, un registro que no se puede leer— pregunta sólo a git, como
        hacía el shell, y se niega si el principal es un proyecto de Dockergento (`fef2d51`):
        falla cerrada, con `--force` como salida.

        Las suites que le preguntaban a `bin/run setup` le preguntan al binario
        (`worktree_test.sh`, `non_interactive_test.sh` y `tests/integration/setup_options_test.sh`,
        que la primera corrida destapó). Con las reparaciones y el árbol ya sin los cuatro ficheros:
        54 fallos antes y después, diff vacío.
  - [x] El mismo paso, para `doctor` — trigésimo primer comando retirado así.
        `console/commands/doctor.sh` (185 líneas) está borrado, con `console/helpers/doctor.sh` y
        las diecisiete comprobaciones de `console/tasks/doctor/`, que sólo él ejecutaba; quedan 34
        ficheros en `console/commands/`. Las diecisiete tienen su par en `app/checks.go`. El handler
        pedía al motor concreto; ahora va por `newEngine` y `doctor_engine_test.go` fija lo que
        pide, el código de salida que decide el diagnóstico y los informes JSON y de texto.
        `checks_test.go` porta lo que sólo fijaban las suites del shell: el veredicto de memoria
        de la VM y la acción por runtime (por debajo de 4 GB es error, como en el shell), que una
        comprobación que entra en pánico es un aviso y las demás siguen, que los avisos solos no
        hacen fallar el resumen, las comprobaciones de proyecto, la de Magento y la entrada de
        hosts que falta. `doctor_contract_test.sh` (la codificación de un resultado en el ejecutor
        del shell) y `vm_resources_test.sh` se van con él: 25 aserciones.

        Las suites le preguntan al binario; `go_passthrough_test.sh` sostiene el número y el orden
        de las diecisiete en vez de compararlas con el shell, y `doctor_test.sh` deja los casos que
        metían comprobaciones en el directorio del ejecutor del shell. Buscando llamadas a
        `bin/run` salieron dos regresiones más de retiradas anteriores, invisibles porque sus
        suites se saltan sin Docker: `tui.sh` y `mcp.sh` pedían `list`, `describe`, `logs`,
        `mysql`, `magento`, `start`, `stop`… a `bin/run`, se tragaban la negativa y enseñaban una
        flota vacía o herramientas rotas (`f3ba798`); y `anonymisation_test.sh` importaba con
        `bin/run mysql`. Con el árbol ya sin los veintidós ficheros: 51 fallos antes y después, diff
        vacío.
  - [x] El mismo paso, para `clean` — trigésimo segundo comando retirado así.
        `console/commands/clean.sh` (335 líneas) está borrado; quedan 33 ficheros en
        `console/commands/`. El colector de Go (`app/collect.go`) no tenía ninguna prueba: la única
        era `clean_test.sh`, que necesita daemon. `collect_test.go` fija sobre la misma escena qué
        selecciona `Survey` y qué borra `Collect` — lo muerto y sus volúmenes y plantilla sí, lo
        vivo y lo suyo nunca, un volumen sin contenedores es no atribuible y no se borra, el
        registro caducado se va y el vivo se queda, mirar no borra nada, ningún puerto puede hacer
        un prune — y `clean_test.go` cuándo recoge el handler: sólo con `--force` o `HM_FORCE`,
        también bajo `--json`, y preguntando sólo con un sí. Cada regla con su mutación.

        Escribirlas destapó un peligro: `Collect` borraba por nombre, y un nombre vacío casaba con
        todos los contenedores sin etiqueta y todos los volúmenes que empiezan por `_`. Ahora un
        nombre vacío no borra nada (`c425248`). La revisión añadió otro: los volúmenes de un entorno
        de rama muerto se borraban por el prefijo `<proyecto>_`, y como Compose admite `_` en los
        nombres de proyecto, recoger un `x` muerto se llevaba los de un `x_algo` vivo; ahora se
        borran por la etiqueta `com.docker.compose.project`. Dos comportamientos heredados del shell quedan en
        la pista C. Nadie llamaba al `.sh`; `clean_test.sh` y `concurrent_safety_test.sh` le
        preguntan al binario. Con el árbol ya sin él: 51 de 1051 aserciones fallaban antes y
        después, diff vacío.
  - [x] `create-project` portado y su `.sh` retirado — primer comando de la pista B.
        `console/commands/create-project.sh` (120 líneas) está borrado, con las dos tareas que sólo
        él usaba, `version_manager.sh` y `magento_installation.sh`; quedan 32 ficheros en
        `console/commands/`. Es `setup` con la versión elegida en vez de leída del `composer.lock`,
        así que reutiliza `app.Setup` (`Decide`, `Save`, `requirementsFor`, `Write`, el proxy y los
        pasos de instalación por el binario) y añade la edición y la versión, `composer
        create-project` en un directorio temporal del contenedor copiado sobre el de trabajo,
        `composer.json` traído al host y `-u` pasado a `install` (`4cef0a9`). Se niega desde un
        worktree sin entorno propio, como `setup`. Diferencias con el shell, decididas: la última
        versión es la mayor por orden de versiones (el `jq 'keys | last'` del shell ordenaba como
        texto y elegía 2.4.9 sobre 2.4.10); una versión no soportada se niega con 2 en vez de volver
        a preguntar; `Decide` pregunta también el capturador de correo; ya no se pregunta si las
        versiones de los servicios convencen, como tampoco lo hace `setup`.

        `write_from_docker-compose_templates.sh`, que sólo cargaba `version_manager.sh`, se queda
        de momento: `mail_catcher_test.sh` genera con él los compose que prueba, y sus reglas no
        están portadas a Go (pista C). Con el árbol ya sin los tres ficheros: 52 de 1054 aserciones
        fallaban antes y después, diff vacío; la que falló de más en la línea base era que
        `create-project` seguía tabulado como `shell` aquí.
  - [x] El baile del vendor de `composer` en macOS portado, y `composer` y `exec` retirados.
        En darwin, `composer install|update|require|remove` caía a `composer.sh`; ahora el binario
        hace los pasos él mismo por la costura del motor — reiniciar `phpfpm`, meter `vendor`,
        ejecutar Composer, meter los ficheros por defecto de Magento, parar `phpfpm`, vaciar el
        `vendor` del host, sacar el directorio de trabajo entero y arrancar `phpfpm` —, parando en
        el primero que falla con su código, y responde a toda invocación de `composer`
        (`28560a5`). Desde un worktree sin entorno propio se niega con la pista del shell
        (`--force` delante de `composer`). `ClearVendor`, el único paso que borra en el host, se
        niega a cualquier ruta que no sea exactamente el `vendor` del proyecto una vez resueltos
        los enlaces y los `..`. `composer_vendor_test.go` fija cada paso, cada fallo y cada
        negativa; `vendor_test.go`, las rutas.

        `console/commands/composer.sh` (121 líneas) y `console/commands/exec.sh` (22), cuyo último
        llamador directo era `composer.sh`, están borrados, con `tests/unit/composer_vendor_test.sh`
        (10 aserciones, portadas); quedan 30 ficheros en `console/commands/`. `go_orchestration_test.sh`
        compara ahora `exec` con el propio Compose y `composer show` con el `exec` que ejecutaba.
        Con el árbol ya sin los tres ficheros: 51 fallos antes y después, diff vacío. Con esto la
        pista A queda vacía.

        Revisado (`review-52a4558955536e15`, tramo `312f323..48a918b`): sin aprobación ni rechazo, con un único
        hallazgo escalado por causalidad desconocida, `R3-missing-vendor-mkdir`: el binario no crea
        `vendor` en el host antes de meterlo en el contenedor, como hacía el `mkdir -p` del shell.
        No cambia nada observable: un `docker cp` de un directorio vacío no borra lo que hay
        dentro, `Transfer.Into` se salta la ruta que no existe (`app/transfer.go:52-55`), Composer
        crea `vendor` en el contenedor y la copia final lo trae al host. Queda anotado, sin cambio.
  - [x] `cloud` y `cloud-login` portados y retirados. Ejecutan la CLI de Magento Cloud en el
        contenedor de PHP: `cloud` con sus argumentos unidos en un `bash -c`, como el shell, y
        `cloud-login` después de decir dónde se genera el token (`25b80c8`); `cloud_test.go` fija
        lo que piden, el estado de la CLI y que fuera de un proyecto no se ejecuta nada.
        `console/commands/cloud.sh` (9 líneas) y `cloud-login.sh` (12) están borrados; quedan 28
        ficheros en `console/commands/`. Siguen en `hm --help` porque están declarados en
        `command_descriptions.json`. Apartados: 51 de 1050 aserciones fallaban antes y después,
        diff vacío.
  - [x] `dbeaver`, `sequelace` y `tableplus` portados y retirados, con un solo handler
        (`7ab593c`): credenciales y puerto publicado de la descripción, la URL como la construía
        `db_client.sh`, y el cliente abierto con `open -a` en macOS o `dbeaver`/`xdg-open` en el
        resto, como un solo argumento y no por `eval` (una contraseña con espacios ya no rompe la
        orden). `--port` sin valor al final es un error de uso, donde el shell entraba en un bucle;
        una credencial vacía cuenta como `magento`. Sus tres ficheros (11 líneas cada uno),
        `console/tasks/db_client.sh` y su prueba unitaria (19 aserciones, portadas a
        `db_client_test.go`) están borrados; quedan 25 ficheros en `console/commands/`.
        `db_client_test.sh` de integración le pregunta al binario. Apartados: 51 fallos antes y
        después, diff vacío.
  - [x] `config-env` portado y retirado (`e553f9c`). Es el paso con el que ya terminaba una
        importación — aplicar `config/docker/local_settings.json`, sembrado desde el de la
        herramienta la primera vez, un `bin/magento` por entrada —, expuesto solo: el motor
        construye el `Importer` en un sitio para los dos. Sin dominio se niega con `hm setup` como
        salida, donde el shell lo preguntaba; un argumento se niega en vez de ignorarse.
        `configure` no tenía prueba y `configure_test.go` fija lo que ejecuta. `config-env.sh` (6
        líneas) está borrado; quedan 24 ficheros. `set_magento_configs.sh` se queda: sólo lo usa
        una prueba de `set_current_domain`, cuyo par en Go no está probado (pista C).

        Buscando llamadores salió una regresión de Linux de una retirada anterior:
        `fix_linux_permissions.sh` ejecutaba `${COMMANDS_DIR}/docker-compose.sh`, retirado en
        `f8df8a2`, y `post-start` fallaba ahí en cada `hm start` en Linux. La búsqueda de
        llamadores directos (`"$COMMANDS_DIR"/<nombre>.sh`) no veía la forma con llaves. Ahora usa
        `$DOCKER_COMPOSE` (`fd41101`), y `no_dead_command_paths_test.sh` falla ante cualquier ruta
        a un script de comando que ya no existe, escrita como sea; destapó además dos
        comprobaciones de suite que leían scripts borrados con `grep … || true` y pasaban sin mirar
        nada. Apartado: 51 fallos antes y después (las cinco aserciones nuevas entraron a mitad de
        la corrida y pasan), diff vacío.
  - [x] `post-start` y `rebuild` portados y retirados (`f4ad14d`). Los pasos de Linux tras
        arrancar — igualar los ids de usuario y grupo de los contenedores con los del host, meter
        las autoridades de certificación y las entradas de autoenrutado en el `/etc/hosts` de
        PHP — corren en Go con las órdenes de contenedor del shell byte a byte (también sus dos
        rarezas, comentadas: `type apk /dev/null` y el `find` de grupo con `-user`), y `Start` y
        `Restart` dejan de devolverle `post-start` al shell. `rebuild` para, levanta con las
        imágenes construidas (`UpBuilding`, el `--build` de Compose), comprueba los bind mounts
        después de cada reconstrucción, como el shell, y corre los mismos pasos; se niega desde un worktree sin entorno
        propio. Ninguna prueba los ejecuta contra un Linux de verdad.

        De paso: `Engine.operator` le pasaba `runtime.GOOS` al operador y sus comprobaciones
        comparan con `mac`, así que en macOS la de que `vendor` no esté montado desde el host no
        corría nunca, ni en `start` ni en `rebuild`; `validate_bind_mounts.sh` sí corría en el
        shell. Vuelve a correr.

        `post-start.sh`, `rebuild.sh`, `fix_linux_permissions.sh`, `set_etc_hosts.sh`,
        `validate_bind_mounts.sh`, `get_bind_mount_path.sh` y sus tres pruebas unitarias (14
        aserciones, portadas) están borrados; quedan 22 ficheros en `console/commands/`. La copia
        de los pasos de Linux que llevaba `rebuild.sh` desaparece con él. Apartados: 51 fallos antes
        y después, diff vacío.
  - [x] `grunt` y `update` portados y retirados (`496aa91`). `grunt` ejecuta sus seis pasos en
        el contenedor de PHP, las órdenes del shell una por `bash -c`, y para en el primero que
        falla; sin tema o sin locale es un error de uso, donde el shell salía con 0. `update` hace
        `git pull` de la instalación de la herramienta y regenera el autocompletado; niega una
        instalación anclada a una etiqueta, una que no es un checkout y un `pull` que falla, con
        los mensajes del shell, y se prueba contra un origen y un clon temporales.
        `version_switching_test.sh` lo ejecuta desde una copia del binario dentro de su clon. Sus
        dos ficheros (38 y 35 líneas) están borrados; quedan 20. Apartados: 51 fallos antes y
        después, diff vacío.
  - [x] `ssl` y `transfer-media` portados y retirados. `ssl` (`667431b`) genera el certificado
        con `mkcert`, instalándolo antes como el shell (brew en macOS, apt y la descarga en el
        resto), lo copia a `hitch` como `/etc/hitch/testcert.pem`, se lo da al usuario `hitch` y
        reinicia con el `restart` de Compose — no con el parar y levantar del operador, que recrea
        un contenedor cuya configuración cambió y el certificado vive en su sistema de ficheros —.
        El certificado se hace en un directorio temporal (el shell dejaba `ssl.pem` en el
        proyecto); se niega antes de tocar el host sin `hitch` corriendo o desde un worktree sin
        entorno propio. Los programas del host van por el puerto `HostPrograms` y ninguna prueba
        los ejecuta. `transfer-media` (`075e00f`) hace el `rsync` por SSH del shell, con sus
        valores de Magento Cloud y sus preguntas; una opción desconocida se niega. Sus dos
        ficheros (65 y 71 líneas) están borrados; quedan 18. Apartados: 51 de 1044 aserciones
        fallaban antes y después, diff vacío.

        Revisado (`review-8ef8376d6e2bb543`): sin aprobación ni rechazo, con un hallazgo escalado por causalidad
        desconocida, `R3-transfer-media-output-drift`: los textos de `transfer-media` no son los del
        shell byte a byte — «Transferring» en vez de la errata «Transfering», «Press enter to
        continue...» en vez de «Press any key continue...» (lo que espera es Intro), sin el espacio
        final del saludo —. Ningún script lee esos textos; queda anotado, sin cambio.
  - [x] `launch` y `permissions` portados y retirados. `launch` (`a85a6af`) abre la dirección
        pedida de la descripción — tienda, admin, correo, RabbitMQ, búsqueda — con `open` o
        `xdg-open`, y bajo `--json` dice dónde sin abrir nada, con los códigos del shell para dos
        destinos, una opción desconocida, una tienda sin dominio y un servicio sin puerto.
        `permissions` (`f612e39`) construye los permisos de un agente desde
        `command_descriptions.json` y `ai-exclusions.json`; bajo `--json` es la configuración sin
        sobre, como el shell. Comparado con el shell sobre los datos reales, su salida JSON y de
        texto es idéntica byte a byte con y sin `--strict`; esa comparación encontró que las notas
        del fichero (claves con `_`) guardan listas, y el primer intento no las apartaba antes de
        leer. Dos suites les preguntaban a `bin/run` (`13cb0fe`) y `validation_cache_test.sh`
        prueba ahora la validación de `bin/run` con `switch` (`3b55b18`). Sus dos ficheros (86 y 91
        líneas) están borrados; quedan 16. Apartados: 48 de 1048 aserciones fallaban antes y
        después (tres menos que antes: las opciones mal formadas de `launch` se niegan ya sin
        Docker), diff vacío.
  - [x] `ai-context` portado y retirado (`878943d`). Escribe el bloque de `AGENTS.md`, un
        `CLAUDE.md` que apunta a él si no hay ninguno y la entrada de la herramienta en `.mcp.json`
        desde la descripción, byte a byte y con los modos que les daba el shell, y niega un bloque
        sin cerrar como él. La única diferencia buscada: la entrada MCP arranca `bin/hm`, que le
        pasa `mcp` al shell, en vez de `bin/run`. La paridad la probó un oráculo que ejecutaba el
        `ai-context.sh` real con sólo `collect_project_info` sustituido, sobre siete escenarios; lo
        que sostenía queda fijado por sí solo en Go, y las comprobaciones de
        `data/ai-exclusions.json` pasan a `ai_exclusions_test.sh` (`be14623`). La huella del
        `doctor` comparte ya el código del escritor y escapa las cadenas como `jq`.
        `ai-context.sh` (106 líneas), `console/tasks/agent_context.sh`, su prueba unitaria y el
        oráculo están borrados; quedan 15. Tres suites que necesitan Docker le preguntan al
        binario. Apartados: 48 fallos antes y después, diff vacío.

        Revisado de `931a66a` a `8008679` (`review-3e499cf488dd1eb6`): sin aprobación ni rechazo,
        con un hallazgo escalado por causalidad desconocida, `R3-launch-permissions-help-refused`:
        `hm launch --help` y `hm permissions --help` contestaban con un error en vez de la ayuda.
        No eran sólo esos dos: ningún comando portado dejaba llegar `--help` a `bin/run`, que es
        quien da la ayuda de cada comando, y `worktree`, `stop` o `magento` respondían como si fuera
        un argumento. El router lo deja pasar ahora para todos (`1801487`), y una prueba lee los
        casos del `switch` de `run.go` para que un comando portado después tampoco se lo quede.
        Antes, `cli_contract_test.sh` le pedía `bash`, ya retirado, a `bin/run` para comprobar el
        error fuera de un proyecto; ahora se lo pide al binario con `magento` (`8008679`), y la
        base baja a 46 fallos.
  - [x] `proxy` terminado y retirado (`65bcb97`). Arriba, abajo y estado ya eran del binario; lo
        único que le quedaba a `proxy.sh` era negar un subcomando desconocido, y ahora lo niega el
        binario con el mismo tipo, mensaje y pista. Su texto de ayuda nunca se mostraba: `bin/run`
        contesta `--help` antes de que corra el comando. `proxy_test.sh` y el experimento de diez
        worktrees le preguntan al binario (`5af2ccb`). `proxy.sh` (123 líneas) está borrado;
        quedan 14. `console/tasks/proxy.sh` se queda: lo carga
        `write_from_docker-compose_templates.sh` (pista C). Apartados: 46 de 1032 aserciones
        fallaban antes y después, diff vacío.
  - [x] `share` portado y retirado (`80ba914`). Abre un túnel rápido de Cloudflare hacia el
        servicio web del proyecto en un contenedor propio sobre su red, y lo sostiene hasta Ctrl-C:
        la misma pregunta que nombra lo que se expone, la cabecera Host reescrita al dominio, sesenta
        segundos para la dirección, los mismos rechazos y el mismo texto. Un puerto nuevo, `Relays`,
        arranca, lee, espera y quita esos contenedores por nombre; `tunnel` lo usará también. Dos
        diferencias: un túnel que no arranca o no da dirección se cierra sin decir además que se
        dejó de compartir, y un `HM_SHARE_PORT` que no es un número se niega. `share_test.sh`, que
        necesita Docker e internet, le pregunta al binario. `share.sh` (138 líneas) está borrado;
        quedan 13. Apartados: 46 de 1034 aserciones fallaban antes y después, diff vacío.
  - [x] `tunnel` portado y retirado (`0c1e262`). Lleva un servicio del proyecto a un puerto libre
        de esta máquina con un relé socat en su red, publicado sólo en 127.0.0.1, y lo sostiene
        hasta Ctrl-C: el único puerto expuesto si no se dice cuál, los mismos rechazos para un
        servicio que falta, que no existe o que está parado y para un puerto ambiguo, veinte
        intentos hasta que el puerto contesta, y `--close` para un servicio o para todos. Reutiliza
        `Relays`; dos puertos pequeños dicen qué servicios definen los ficheros y qué puertos
        expone un contenedor. Dos diferencias: un puerto que no es un número se niega, y un relé que
        falla se cierra sin decir además que el túnel se cerró. `proxy_test.sh` le pregunta al
        binario (`34cca7e`). `tunnel.sh` (175 líneas) está borrado; quedan 12. Apartados: 46 de
        1036 aserciones fallaban antes y después, diff vacío.

        Revisado de `8008679` a `e899801` (`review-226853e2db9f5747`): sin aprobación ni rechazo, con tres
        hallazgos escalados por causalidad desconocida, los tres ciertos. Un Ctrl-C mientras
        `share` esperaba su dirección mataba el proceso y dejaba el entorno en internet: el manejador
        de señales se instalaba sólo después de abrir (`R3-share-interrupt-during-address-wait-leaves-public-tunnel`,
        `R4-share-ctrlc-during-open-leaks-public-tunnel`). Y `tunnel` cerraba su relé, y quitaba uno
        viejo, por prefijo, así que el túnel a `db` se llevaba el de `db-replica`
        (`R4-tunnel-exit-closes-sibling-service-tunnels`; el shell hacía lo mismo al abrir).
        Arreglado en `63a62fa`: la interrupción se toma desde antes de preguntar, en tres fases
        —en la pregunta termina, mientras abre cierra dos veces y termina con 130, mientras sostiene
        sólo cierra—, `share` deja de esperar a un relé que ya no se puede leer, y el relé de un
        servicio se busca por su nombre exacto.
  - [x] `compatibility` portado y retirado (`0907b84`). La tabla de versiones de Magento sale de
        `requirements.json` y `equivalent_versions.json` idéntica byte a byte a la del shell, con y
        sin color, sobre los datos reales; la fija una fixture que es la salida del propio
        `compatibility.sh` sobre datos elegidos por los órdenes que le daban `sort -V` y el
        `sort_by` de jq. La primera corrida con el fichero apartado no valió: la guardia de rutas
        muertas encontró que `install`, ante una versión no soportada, ejecutaba el script por su
        ruta desde `input_info.sh`; ahora se lo pide al binario (`fe2ebf2`). Dos suites dejaron de
        contar con que el comando caía al shell (`c295096`). `compatibility.sh` (162 líneas) está
        borrado; quedan 11. Apartados: 46 de 1038 aserciones fallaban antes y después, diff vacío.
  - [x] `transfer-db` portado y retirado (`15eb172`, escrito por un subagente en su propio
        worktree e integrado aquí). Las mismas preguntas y valores por defecto, el mismo `ssh` con
        `mysqldump` remoto, la copia al contenedor de la base de datos, la importación, la
        anonimización y los pasos de Magento, con los mismos rechazos. Diferencias buscadas: el
        rechazo de un servicio parado puede darse ya (el shell moría antes con el error de
        Compose), la contraseña no se muestra como valor por defecto, y una anonimización que falla
        lo dice. Al portarlo apareció que `ask` perdía las respuestas encadenadas por tubería tras
        la primera: lo arregla `727d849` para todos los comandos. `transfer-db.sh` (217 líneas)
        está borrado; quedan 10. Apartados: 46 de 1040 aserciones fallaban antes y después, diff
        vacío.
  - [x] `switch` y `verify` portados y retirados (`b051bfb`, `32d5cda`, escritos por un subagente
        en su propio worktree e integrados aquí). `switch` lista y cambia la versión instalada de la
        herramienta —etiqueta, rama, `--stable`— con los mismos rechazos ante una referencia
        desconocida, cambios sin confirmar o una instalación que no es un checkout; comparado con el
        shell en clones temporales, stdout, stderr y código coinciden. `verify` corre las
        comprobaciones del proyecto dentro del contenedor php sobre los ficheros cambiados (un puerto
        nuevo, `Changes`, los pide a git). Diferencias buscadas: una opción desconocida se niega antes
        de mirar Docker, la salida de una comprobación fallida se imprime línea a línea (el shell la
        pegaba en una) y tal cual (el shell la pasaba como formato a `printf`). Su ejecución real
        dentro de un contenedor no se ha probado aquí: `verify_test.sh` necesita Docker.
        `validation_cache_test.sh` usa ahora `mcp --config` como comando del shell sin proyecto
        (`957f2b3`). `switch.sh` (227 líneas) y `verify.sh` (234) están borrados; quedan 8.
        Apartados: 46 de 1042 aserciones fallaban antes y después, diff vacío.
  - [x] `mcp`, `tui`, `install` y `ai-doctor` portados y retirados (`13a87f6`, `05bbbda`,
        `0e0ced6`, `9d31117`), cada uno escrito por un subagente en su propio worktree e integrado
        aquí. `mcp` responde byte a byte lo que respondía `mcp.sh` —lo fija un golden que es la
        salida del `bin/run mcp` real sobre 70 peticiones—, y su `--config` apunta ya a `bin/hm`;
        sus reglas de SQL de sólo lectura y de rutas de configuración pasan a Go (`165d35a`). `tui`
        dibuja el mismo panel con las mismas teclas y lanza los mismos comandos en el directorio de
        cada entorno; sin terminal se niega como el shell. `install` hace las mismas preguntas y el
        mismo `setup:install` argumento por argumento, con el mismo arranque del administrador; un
        valor de `data/config.json` con espacios llega ya como un solo argumento. `ai-doctor` da la
        misma tabla y los mismos estados, con un digest en Go probado contra un golden del shell.
        Las suites que leían el código de los scripts fijan ahora esas reglas en Go, y se retiraron
        con ellos `admin_bootstrap.sh`, `tui_frame.sh`, `tui_render.sh` y `mcp_tools.sh` con sus
        pruebas unitarias (`faedfe5`–`e82f47a`, `165d35a`). La primera corrida con los ficheros
        apartados encontró esas suites; la guardia de que `tui` y `mcp` llaman al binario mira ya
        sólo los scripts que quedan (`bff441f`). `mcp.sh` (327 líneas), `tui.sh` (261),
        `install.sh` (248) y `ai-doctor.sh` (168) están borrados; quedan 4. Apartados: 46 fallos
        antes y después, diff vacío.
  - [x] `db` terminado y retirado (`b398205`, escrito por un subagente en su propio worktree e
        integrado aquí). Sus nueve subcomandos ya eran del binario; lo único que le quedaba a
        `db.sh` era el texto de uso y la negativa de un subcomando desconocido, y ahora los da el
        binario, idénticos byte a byte al script ejecutado directamente. El binario enruta todo
        `db`, y la excepción de comando a medias de `migration_status_test.sh` desaparece. Tres
        suites que necesitan Docker le preguntan al binario (`92499cf`); `validation_cache_test.sh`
        sigue usando `bin/run db list` para probar la validación del propio `bin/run`, y queda para
        la pista C. `db.sh` (709 líneas) está borrado; quedan 3. Apartados: 46 de 895 aserciones
        fallaban antes y después, diff vacío.

        Revisado de `e899801` a `92499cf`. El tramo entero no cabía en el presupuesto de las
        lentes, así que se revisó en ocho tramos, cada uno en un worktree fijado en su último
        commit: los siete aprobados y reconocidos son `review-1bea71db957e3fa8` (interrupciones de
        `share`/`tunnel`), `review-31d312a9aedac743` (`compatibility`), `review-95806b82467a9a41`
        (`transfer-db`), `review-760af4943b010ecf` (`switch` y `verify`), `review-3e7cb01e4e30d7ac`
        (`mcp`), `review-5a1b449338d207d7` (`install`) y `review-16e898c648f3faa5` (`ai-doctor` y
        `db`). El de `tui` (`review-aa109a427cb0865e`) acabó sin aprobación ni rechazo, con un
        hallazgo escalado, `R3-sigint-during-handed-action`: qué pasa con el panel y la terminal si
        llega una señal mientras un comando lanzado tiene la terminal. Ese hallazgo y los avisos que
        lo merecen —un Ctrl-C en la pregunta de `share` que cerraría un túnel de otra terminal, los
        ficheros sin entrecomillar de `verify`, el filtro de SQL de `mcp`— se arreglan aparte.
  - [x] `ai-init`, `ai-pull` y `ai-reset` portados y retirados (`074e8ee`, `6da6b3f`, `8e7fd3e`,
        escritos por un subagente en su propio worktree e integrados aquí): los últimos tres. Los
        mismos ficheros escritos —contenido y modos—, el mismo registro con sus sumas, el mismo
        borrado por lista blanca y las mismas salidas, comparados con el shell en 38 casos con un
        `curl` falso que servía un tarball local. Peculiaridades del shell conservadas: `ai-pull
        --force` no llega nunca al comando (el router se queda `--force` como opción global) y
        `ai-init` no instala las skills de la propia herramienta. Con ellos se van las tareas
        `ai_download.sh`, `ai_extract.sh`, `ai_registration.sh` y `ai_wizard.sh` y el helper
        `digest.sh`. La primera corrida con los ficheros apartados encontró que, sin ningún script,
        git borra `console/commands/` y la ayuda general dejaba de listar comandos; ahora sólo los
        comandos propios de un proyecto necesitan ese directorio (`7b95928`). `ai-init.sh` (364
        líneas), `ai-pull.sh` (299) y `ai-reset.sh` (321) están borrados; no queda ninguno: los 65
        comandos los responde el binario. Apartados: 46 de 897 aserciones fallaban antes y
        después, diff vacío.

Antes de la fase 5 hay una puerta: las cuatro medidas de la Fase 1 (§7 del documento de
arquitectura). Las fases 2, 3 y 4 no dependen de ellas.

## Lo que va costando cada comando portado

Medido en esta máquina, misma salida byte a byte:

| | bash | go |
|---|---|---|
| `list --json` | 205 ms | **63 ms** |
| `describe --json` | 285 ms | **94 ms** |
| `doctor --json` | 260 ms | **85 ms** |
| `start` (crear el entorno) | 555 ms | **265 ms** |
| `start` (entorno ya en marcha) | 320 ms | **95 ms** |
| `stop` (parar el entorno) | 10,3 s | 10,2 s — lo que tarda es el contenedor |
| `magento <lo que sea>` | 210 ms | **75 ms** de envoltorio |

De dónde sale: leer la configuración de compose con librería en vez de `docker compose config`
son 1,7 ms contra 58 ms, y las preguntas independientes —la rama de cada entorno, la versión de
git, la de compose, el exec de xdebug— se hacen a la vez en lugar de en fila. Eso último es lo que
bash no puede hacer.

Lo que cuesta: enlazar el motor de Compose sube el binario publicado de **8,5 MB a 60,6 MB**, y el
registro en SQLite (Go puro, sin cgo) lo deja en **64,5 MB** y el
grafo de dependencias de 70 módulos a 426. Es el precio de ADR-009 bis y está aceptado a
conciencia; el detalle y lo que se compra con ello, en `docs/research/2.0-arquitectura.md`.

`db` son dos familias que comparten nombre: **instantáneas** (volcados en fichero, para guardar) y
**plantillas** (copias byte a byte del directorio de datos en un volumen, para levantar entornos en
segundos). Las dos familias van por Go, y con ellas el texto de uso y la negativa a un subcomando
desconocido.

`composer install|update|require|remove` sigue en shell **en macOS**: ahí no se ejecuta en el
contenedor sin más, sino que se copia el `vendor` dentro, corre Composer y se copia el árbol
entero de vuelta encima del host —borrando su `vendor` por el camino—. La razón es
`mirrorsVendor` (`internal/cli/php.go:98`): en darwin esas cuatro invocaciones son una secuencia
de copiar, ejecutar y copiar de vuelta, no una sola llamada, y no es cosa de portar a medias.
Todo lo demás de `composer` y `magento` va por Go en las dos plataformas.

`start` y `restart` van por Go **en las dos plataformas**. Lo que en Linux hay que hacer después
—igualar los ids de usuario y grupo del contenedor con los del host, y escribir los dominios del
proyecto en su `/etc/hosts`— se devuelve al shell como `hm post-start`, que es un comando y no un
bloque dentro de `start`: hay dos cosas que levantan entornos ahora, y una sola copia de esos
pasos. Sigue en shell porque el segundo lee los dominios de la base de datos con `hm mysql`, que
es tanda 3: portarlo ahora sería portar aquello primero.

## Los comandos

Tanda 1 son los del día a día; 2 los que tocan estado; 3 el resto; 4 los que orquestan
herramientas externas, que son los últimos por coste-beneficio y no por estar exentos.

| Comando | Área | Tanda | Implementación |
|---|---|---|---|
| `ai-context` | ai | 4 | go |
| `ai-doctor` | ai | 4 | go |
| `ai-init` | ai | 4 | go |
| `ai-pull` | ai | 4 | go |
| `ai-reset` | ai | 4 | go |
| `bash` | tools | 3 | go |
| `clean` | environment | 2 | go |
| `cloud` | tools | 4 | go |
| `cloud-login` | tools | 4 | go |
| `compatibility` | tools | 4 | go |
| `composer` | magento | 1 | go |
| `config-env` | tools | 3 | go |
| `copy-from-container` | files | 3 | go |
| `copy-to-container` | files | 3 | go |
| `create-project` | tools | 3 | go |
| `db` | database | 2 | go |
| `dbeaver` | database | 3 | go |
| `debug-off` | tools | 3 | go |
| `debug-on` | tools | 3 | go |
| `describe` | environment | 1 | go |
| `docker-compose` | tools | 3 | go |
| `docker-stop-all` | tools | 3 | go |
| `doctor` | environment | 1 | go |
| `down` | environment | 2 | go |
| `exec` | tools | 1 | go |
| `grunt` | magento | 4 | go |
| `install` | magento | 3 | go |
| `launch` | environment | 3 | go |
| `list` | environment | 1 | go |
| `logs` | environment | 1 | go |
| `magento` | magento | 1 | go |
| `masquerade` | database | 3 | go |
| `mcp` | ai | 3 | go |
| `mysql` | database | 3 | go |
| `mysqldump` | database | 3 | go |
| `n98-magerun` | magento | 3 | go |
| `npm` | magento | 3 | go |
| `post-start` | environment | 3 | go |
| `permissions` | ai | 3 | go |
| `proxy` | environment | 2 | go |
| `purge` | magento | 3 | go |
| `rebuild` | environment | 3 | go |
| `restart` | environment | 1 | go |
| `sequelace` | database | 3 | go |
| `web` | tools | 1 | go |
| `set-host` | tools | 3 | go |
| `setup` | environment | 2 | go |
| `share` | environment | 3 | go |
| `ssl` | tools | 4 | go |
| `start` | environment | 1 | go |
| `stop` | environment | 1 | go |
| `switch` | versions | 3 | go |
| `tableplus` | database | 3 | go |
| `test-integration` | magento | 3 | go |
| `test-unit` | magento | 3 | go |
| `transfer-db` | database | 4 | go |
| `transfer-media` | files | 4 | go |
| `tui` | environment | 3 | go |
| `tunnel` | environment | 3 | go |
| `update` | versions | 3 | go |
| `varnish-off` | tools | 3 | go |
| `varnish-on` | tools | 3 | go |
| `verify` | tools | 3 | go |
| `version` | versions | 3 | go |
| `worktree` | environment | 2 | go |

## Cómo se porta un comando

1. Se escribe el caso de uso en `internal/app`, contra puertos, con sus tests y sin Docker.
2. Se escribe el adaptador que haga falta en `internal/adapters`.
3. Se conecta en `internal/cli` (el puente ya no existe).
4. Se porta el test de integración del shell, que pasa a ejecutar el binario.
5. Se marca aquí, en la tabla y en el contador de arriba.

El test `tests/unit/migration_status_test.sh` comprueba que esta tabla no miente en ninguna
dirección: que están todos los comandos y que lo marcado como Go existe de verdad en el árbol
de Go, y también al revés — que todo lo que el router ya resuelve está marcado `go` aquí, sin
excepciones.
