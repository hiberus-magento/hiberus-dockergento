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
| Comandos en Go | 16 de 65 |
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

**El andamio, y cuándo se cae.** Hay dos entradas que no son comandos —no salen en `--help` ni
están en `command_descriptions.json`, y llevan guion bajo porque es lo que ese fichero ya usa para
las claves que no son comandos—. Existen porque, mientras la disciplina sea paridad byte a byte,
un comando portado no puede reportar nada que el de bash no reporte, así que lo que sólo sabe la
capa Go no tiene dónde mirarse. Cada una muere en un sitio concreto:

| | qué enseña | desaparece cuando |
|---|---|---|
| `hm _binary` | qué build del binario corre | se porte `version`, y pase a ser un campo suyo |
| `hm _registry` | lo que guarda el registro | el registro sea la fuente viva, y `hm worktree list` lo enseñe |

El código está en dos mitades. `dockergento/` es la herramienta como librería —dominio, puertos,
casos de uso, adaptadores y una fachada— y es pública porque `internal/` es una regla del lenguaje
que impide importarla desde otro módulo. `internal/cli/` es la terminal, y es lo único privado:
importa la fachada y los tipos del dominio, y nada más. Ahí es donde entrarán HTTP y MCP, como dos
puertas más sobre las mismas llamadas.

Lo siguiente por hacer está siempre en la primera fase sin terminar de la lista de abajo, y el
detalle de cada una en `openspec/changes/`.

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
- [ ] **3 · Adaptador de Docker (SDK) + tanda 1** — *en marcha*, donde gana todo el equipo
  - [x] Adaptador del demonio por SDK, resolviendo el socket desde el contexto de Docker
  - [x] `list` — el primero: sólo lectura, puro Docker, salida idéntica byte a byte
  - [x] `describe` — el más usado y el de contrato más rico
  - [x] `doctor` — diecisiete comprobaciones, cinco a Docker y cuatro a la máquina
  - [x] `start`, `stop`, `restart`, `logs`, `exec` — Compose como librería (ADR-009 bis)
  - [x] `magento`, `composer` — el baile del vendor en macOS sigue en shell
- [ ] **4 · Registro SQLite con las dos topologías + tanda 2** — *en marcha*
  - [x] Esquema, slots atómicos e importación de lo que escribió bash
  - [x] `mysql` entero, import incluido — y con él consultar, alimentar un contenedor, lanzar uno
        suelto, el indicador de progreso y preguntar por terminal, que necesita el resto de la tanda
  - [x] `worktree` entero — `add`, `list` y `remove`. La registración y el overlay que escribe son
        byte a byte los de bash, comprobado
  - [ ] el registro pasa a ser la fuente viva: cambiar el adaptador de JSON a SQLite de una vez,
        que ya se puede porque los tres subcomandos de `worktree` son de la misma implementación
  - [~] `db` — la mitad de plantillas (`freeze`, `templates`, `clone`, `drop`), que es de la que
        depende `worktree`; las instantáneas siguen en shell
  - [x] `clean` — y con él ya nadie más escribe registraciones desde bash salvo `worktree.sh`
  - [ ] `proxy`, `setup`, `down`
- [ ] **5 · Servicios compartidos, seed, worktrees, GC** — donde gana el trabajo con agentes
- [ ] **6 · Adaptadores de agente: `--json`, MCP, HTTP para la web** — *empezada*
  - [x] `hm web`: la API HTTP y la interfaz de navegador, sobre las mismas llamadas que la CLI
  - [ ] MCP en Go
- [ ] **7 · Tanda 3 y tanda 4: hasta que no quede ningún comando en shell**

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
entero de vuelta encima del host —borrando su `vendor` por el camino—. Depende de
`copy-to-container`, que no está portado, y no es cosa de portar a medias. Todo lo demás de
`composer` y `magento` va por Go en las dos plataformas.

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
| `copy-from-container` | files | 3 | shell |
| `copy-to-container` | files | 3 | shell |
| `create-project` | tools | 3 | shell |
| `db` | database | 2 | shell |
| `dbeaver` | database | 3 | shell |
| `debug-off` | tools | 3 | shell |
| `debug-on` | tools | 3 | shell |
| `describe` | environment | 1 | go |
| `docker-compose` | tools | 3 | shell |
| `docker-stop-all` | tools | 3 | shell |
| `doctor` | environment | 1 | go |
| `down` | environment | 2 | shell |
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
| `mysqldump` | database | 3 | shell |
| `n98-magerun` | magento | 3 | shell |
| `npm` | magento | 3 | shell |
| `post-start` | environment | 3 | shell |
| `permissions` | ai | 3 | shell |
| `proxy` | environment | 2 | shell |
| `purge` | magento | 3 | shell |
| `rebuild` | environment | 3 | shell |
| `restart` | environment | 1 | go |
| `sequelace` | database | 3 | shell |
| `web` | tools | 1 | go |
| `set-host` | tools | 3 | shell |
| `setup` | environment | 2 | shell |
| `share` | environment | 3 | shell |
| `ssl` | tools | 4 | shell |
| `start` | environment | 1 | go |
| `stop` | environment | 1 | go |
| `switch` | versions | 3 | shell |
| `tableplus` | database | 3 | shell |
| `test-integration` | magento | 3 | shell |
| `test-unit` | magento | 3 | shell |
| `transfer-db` | database | 4 | shell |
| `transfer-media` | files | 4 | shell |
| `tui` | environment | 3 | shell |
| `tunnel` | environment | 3 | shell |
| `update` | versions | 3 | shell |
| `varnish-off` | tools | 3 | shell |
| `varnish-on` | tools | 3 | shell |
| `verify` | tools | 3 | shell |
| `version` | versions | 3 | shell |
| `worktree` | environment | 2 | go |

## Cómo se porta un comando

1. Se escribe el caso de uso en `internal/app`, contra puertos, con sus tests y sin Docker.
2. Se escribe el adaptador que haga falta en `internal/adapters`.
3. Se conecta en `internal/cli` y se quita del puente.
4. Se porta el test de integración del shell, que pasa a ejecutar el binario.
5. Se marca aquí, en la tabla y en el contador de arriba.

El test `tests/unit/migration_status_test.sh` comprueba que esta tabla no miente: que están todos
los comandos y que lo marcado como Go existe de verdad en el árbol de Go.
