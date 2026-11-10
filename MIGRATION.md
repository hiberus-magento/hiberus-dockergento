# Migración a Go — dónde estamos

> **El objetivo, para que no se pierda entre tandas:** que no quede ningún comando en shell, que
> la CLI, la TUI y la web se alimenten de las mismas piezas, que no haya nada duplicado y que la
> arquitectura hexagonal se sostenga de verdad. El orden entre tandas se optimiza; el destino no
> se negocia. Lo único que seguirá siendo shell son los comandos que un proyecto añada en
> `config/hm/commands`, que por eso el puente a bash no desaparece.
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
| Comandos en Go | 34 de 65 |
| El binario | `go build -o bin/hm ./cmd/hm` |
| La suite | `go test ./...` y `./tests/run.sh` |

## Cómo continuar si esta sesión se corta

```bash
git checkout release/2.0.0
go build -o bin/hm ./cmd/hm    # el binario
go test ./...                  # los tests de Go
./tests/run.sh tests/unit      # la suite de bash, la parte rápida
./bin/hm hm-go-project         # lo que la capa Go resuelve aquí
```

**El andamio, y cuándo se cae.** Queda una entrada que no es comando —no salen en `--help` ni
están en `command_descriptions.json`, y llevan guion bajo porque es lo que ese fichero ya usa para
las claves que no son comandos—. Existen porque, mientras la disciplina sea paridad byte a byte,
un comando portado no puede reportar nada que el de bash no reporte, así que lo que sólo sabe la
capa Go no tiene dónde mirarse. Cada una muere en un sitio concreto:

| | qué enseña | desaparece cuando |
|---|---|---|
| `hm _registry` | lo que guarda el registro | nada en shell necesite leerlo: hoy es también por donde lo lee `bin/run` |

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
        que depende `down`. Lo único que sigue en shell es el texto de uso
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
segundos). Las plantillas van por Go; las instantáneas siguen en shell. La frontera está entre dos
operaciones independientes, no por la mitad de una.

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
| `ai-context` | ai | 4 | shell |
| `ai-doctor` | ai | 4 | shell |
| `ai-init` | ai | 4 | shell |
| `ai-pull` | ai | 4 | shell |
| `ai-reset` | ai | 4 | shell |
| `bash` | tools | 3 | go |
| `clean` | environment | 2 | go |
| `cloud` | tools | 4 | shell |
| `cloud-login` | tools | 4 | shell |
| `compatibility` | tools | 4 | shell |
| `composer` | magento | 1 | go |
| `config-env` | tools | 3 | shell |
| `copy-from-container` | files | 3 | go |
| `copy-to-container` | files | 3 | go |
| `create-project` | tools | 3 | shell |
| `db` | database | 2 | shell |
| `dbeaver` | database | 3 | shell |
| `debug-off` | tools | 3 | go |
| `debug-on` | tools | 3 | go |
| `describe` | environment | 1 | go |
| `docker-compose` | tools | 3 | go |
| `docker-stop-all` | tools | 3 | go |
| `doctor` | environment | 1 | go |
| `down` | environment | 2 | go |
| `exec` | tools | 1 | go |
| `grunt` | magento | 4 | shell |
| `install` | magento | 3 | shell |
| `launch` | environment | 3 | shell |
| `list` | environment | 1 | go |
| `logs` | environment | 1 | go |
| `magento` | magento | 1 | go |
| `masquerade` | database | 3 | go |
| `mcp` | ai | 3 | shell |
| `mysql` | database | 3 | go |
| `mysqldump` | database | 3 | go |
| `n98-magerun` | magento | 3 | go |
| `npm` | magento | 3 | go |
| `post-start` | environment | 3 | shell |
| `permissions` | ai | 3 | shell |
| `proxy` | environment | 2 | shell |
| `purge` | magento | 3 | go |
| `rebuild` | environment | 3 | shell |
| `restart` | environment | 1 | go |
| `sequelace` | database | 3 | shell |
| `web` | tools | 1 | go |
| `set-host` | tools | 3 | go |
| `setup` | environment | 2 | go |
| `share` | environment | 3 | shell |
| `ssl` | tools | 4 | shell |
| `start` | environment | 1 | go |
| `stop` | environment | 1 | go |
| `switch` | versions | 3 | shell |
| `tableplus` | database | 3 | shell |
| `test-integration` | magento | 3 | go |
| `test-unit` | magento | 3 | go |
| `transfer-db` | database | 4 | shell |
| `transfer-media` | files | 4 | shell |
| `tui` | environment | 3 | shell |
| `tunnel` | environment | 3 | shell |
| `update` | versions | 3 | shell |
| `varnish-off` | tools | 3 | go |
| `varnish-on` | tools | 3 | go |
| `verify` | tools | 3 | shell |
| `version` | versions | 3 | go |
| `worktree` | environment | 2 | go |

## Cómo se porta un comando

1. Se escribe el caso de uso en `internal/app`, contra puertos, con sus tests y sin Docker.
2. Se escribe el adaptador que haga falta en `internal/adapters`.
3. Se conecta en `internal/cli` y se quita del puente.
4. Se porta el test de integración del shell, que pasa a ejecutar el binario.
5. Se marca aquí, en la tabla y en el contador de arriba.

El test `tests/unit/migration_status_test.sh` comprueba que esta tabla no miente en ninguna
dirección: que están todos los comandos y que lo marcado como Go existe de verdad en el árbol
de Go, y también al revés — que todo lo que el router ya resuelve está marcado `go` aquí, con la
única excepción fijada de `db` y `proxy`, que sólo están portados a medias.
