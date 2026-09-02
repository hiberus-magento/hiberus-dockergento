# Investigación: qué robar de gentle-ai y engram (migración a Go)

> Estado: **catálogo priorizado. Nada implementado.**
> Destinatario: la sesión que desarrolla la migración a Go. El estado de la migración está en
> [MIGRATION.md](../../MIGRATION.md); las decisiones, en [2.0-arquitectura.md](2.0-arquitectura.md).
>
> La beta 2.0 no sale hasta que la migración esté madura, así que **aquí no hay nada urgente por
> fecha**. Sí hay tres cosas (GO-01, GO-02, GO-03) que son fallos ya presentes en el árbol, y una
> (GO-22) que es mucho más barata ahora, con 10 comandos portados, que con 64.

---

## 0. Qué se analizó y para qué

Dos herramientas CLI en Go de Gentleman-Programming, clonadas y leídas completas el 2026-09-01:

| | LOC Go | Qué es | Por qué mirarla |
|---|---|---|---|
| [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) | 370.356 | Instalador/orquestador de ecosistemas para agentes de IA | Su cadena de distribución (install script, canales, firma, release gateado) es la más completa que he visto en un CLI Go de este tamaño |
| [engram](https://github.com/Gentleman-Programming/engram) | 138.588 | Memoria persistente para agentes, binario único | Distribución minimalista que funciona, y dos piezas concretas robables |

El disparador fue el one-liner de instalación de gentle-ai:

```bash
curl -fsSL https://raw.githubusercontent.com/Gentleman-Programming/gentle-ai/main/scripts/install.sh | bash
```

**Conclusión de una línea**: cópiales el empaquetado, **no** la arquitectura. En distribución van
muy por delante; en organización del código Go, Dockergento va por delante de los dos.

---

## 1. Encuadre: qué NO copiar

Antes del catálogo, porque ahorra tiempo:

- **No copiar la organización de paquetes.** `internal/cli` de gentle-ai es un god-package de más
  de 400 ficheros planos, con `review_facade.go` de **143,8 KB**, `run.go` de 106,5 KB, `sync.go`
  de 79,6 KB y un test de 202,5 KB. `engram` mete parsing de argumentos y lógica de negocio en el
  mismo `cmd/engram/main.go`, de **3.264 líneas**, y tiene `internal/store/store.go` de 9.078
  líneas. Ninguno de los dos tiene capa de dominio ni puertos. Detalle y comparación en §7.
- **No copiar Homebrew.** Los dos publican formula a un tap propio vía GoReleaser (`brews:`). Es
  un gran gancho de DX y **no encaja con nosotros**: un formula instala un binario suelto, y
  nuestra instalación *es* el checkout git — es lo que hace posibles `hm switch` y `hm update`.
  Descartado con motivo, no por olvido.
- **No copiar los "ratchets" de proceso propios de su dominio** (`.refusal-ratchet-baseline.txt`,
  104 KB de cadenas de rechazo; `.guard-population-baseline.txt`). Sólo tienen sentido en una
  herramienta cuyo producto son prompts. El de deadcode sí sirve, y está en GO-23.
- **No copiar la firma con minisign** (gentle-ai firma `checksums.txt` con minisign y ata la firma
  al `repo=…;tag=…` en el comentario firmado, para que una firma válida no pueda replicarse de otra
  release). Es endurecimiento de cadena de suministro correcto y desproporcionado para una
  herramienta interna. La *idea* —que la firma lleve dentro la identidad de la release— queda
  anotada por si algún día se publica fuera de Hiberus.

---

## 2. Lo que ya está roto en el árbol

Esto no viene de ellos. Salió de comparar su cadena de distribución con la nuestra.

### GO-01 · El binario no sigue al checkout — **el importante**

**Esfuerzo**: M. **Extiende**: REL-02 (`hm switch`), REL-03 (`hm update`).

Nuestro propio [.goreleaser.yaml](../../.goreleaser.yaml) lo escribió en el comentario de cabecera:

> *"el binario tiene que seguir al checkout. Cambiar de versión tiene que traer el binario
> correspondiente, o las dos mitades de la herramienta dejan de estar de acuerdo."*

Nadie lo implementó. Evidencia:

| Sitio | Qué hace | Qué falta |
|---|---|---|
| `installer.sh:101` | descarga `releases/latest/download/hm_${system}_${arch}` | **siempre latest**, sin mirar el tag del checkout |
| `console/commands/update.sh:29-30` | `git pull` + `generate_completion.sh` | no toca el binario |
| `console/commands/switch.sh:166-170` | `git checkout --quiet "$reference"` | no toca el binario |
| `.gitignore:15` | `/bin/hm` ignorado | por eso `git checkout` lo deja intacto, y en silencio |

El mecanismo completo: `bin/hm` es un artefacto descargado, ignorado por git, dentro de un
checkout que sí cambia de versión. Cambiar de versión mueve una mitad y deja la otra.

Con 10 comandos en Go ya es explotable: `hm switch 1.5.0-rc.3` deja un binario 2.0 ejecutando
`list`, `describe`, `doctor`, `start`, `stop`, `restart`, `logs`, `exec`, `magento` y `composer`
en Go contra un checkout 1.5 — y esos comandos ya no pasan por el puente a bash, así que no hay
red. El riesgo crece con cada comando portado.

**Qué hacer**: una función compartida —`console/tasks/fetch_binary.sh`— invocada por `installer.sh`,
`update.sh` y `switch.sh`, que resuelva la referencia actual del checkout y traiga *el binario de
esa referencia*. Cuatro requisitos que no son obvios:

1. **Descargar por tag, no por `latest`**: `releases/download/<tag>/hm_${os}_${arch}`.
2. **Una referencia sin binario no es un error**: cualquier tag anterior a la 2.0 no tiene capa
   Go. En ese caso hay que **borrar** `bin/hm` y repuntar el enlace a `bin/run`. Si no, `hm switch
   1.5.0` deja el binario 2.0 sirviendo un árbol 1.5, que es peor que no cambiar de versión.
3. **Reapuntar el symlink** `/usr/local/bin/hm` según el resultado (`bin/hm` o `bin/run`), porque
   hoy el instalador lo fija una vez y nunca más.
4. **Descarga atómica**: `installer.sh:96-101` ya lo hace bien (baja a `hm.download` y renombra al
   final, para que una descarga interrumpida no parezca un punto de entrada). Mantener ese patrón
   en la función compartida.

### GO-02 · `installer.sh` no es idempotente

**Esfuerzo**: S. Se resuelve dentro de GO-04.

- `installer.sh:71` — `if [ ! -e ~/hm ]` : si el directorio existe, no clona **ni actualiza**.
- `installer.sh:128` — `if [ ! -e /usr/local/bin/hm ]` : si el enlace existe, no lo toca, aunque
  apunte al sitio equivocado.

Relanzar el one-liner sobre una máquina que ya tiene 1.x **no actualiza nada y termina en verde**.
Para un rollout de beta es el peor fallo posible: silencioso. El instalador de gentle-ai trata la
instalación previa como camino de *upgrade*, y "ya estás en la última" como éxito, no como error.

### GO-03 · `checksums.txt` se publica y nunca se verifica

**Esfuerzo**: S. Se resuelve dentro de GO-04.

`.goreleaser.yaml:56` publica `checksums.txt` en cada release. `installer.sh:86-105` descarga el
binario y **no lo verifica**. El manifiesto ya está ahí; sólo falta usarlo.

Su implementación, que es la que hay que copiar (`scripts/install.sh` de gentle-ai, función
`install_binary`): descarga `checksums.txt`, hace `grep` del nombre del archivo, compara con
`sha256sum` o `shasum -a 256` (fallback, porque macOS no trae el primero), y **aborta si no
cuadra, si el archivo no aparece en el manifiesto, o si no hay herramienta de hash**. Escape hatch
explícito `--insecure` que avisa por cada caso. *Fail closed*, no *fail open*.

Añaden también un cinturón: si el archivo descargado pesa menos de 1000 bytes, abortan — es cómo
se detecta que la URL devolvió una página de 404 en vez del binario. Nosotros usamos `curl -fsSL`,
que ya falla en 404, así que ese cinturón es redundante para nosotros.

---

## 3. Distribución e instalación

### GO-04 · `scripts/install.sh`, reescrito

**Esfuerzo**: M. **Absorbe**: GO-02, GO-03. **Depende de**: GO-01.

Un solo script, autocontenido, `set -euo pipefail`, seguro bajo `curl | bash`. Referencia:
`gentle-ai/scripts/install.sh`, 700 líneas bien pensadas. Lo que hay que llevarse, además de
GO-02 y GO-03:

- **`--help`** con ejemplos, y salida limpia. Hoy el instalador no acepta ni una opción.
- **`verify_installation`** como paso final explícito: `hash -r`, ejecuta el binario, comprueba que
  está en el `PATH`, y si no lo está imprime el `export PATH=…` exacto. Además busca en ubicaciones
  conocidas si no aparece en el `PATH`, para poder decir "está instalado aquí, pero no lo alcanzas"
  en vez de "no pude verificar".
- **Mensajes de error que llevan el comando siguiente.** Su `print_homebrew_failure_help()`
  clasifica la salida del fallo y escupe los comandos exactos a ejecutar, incluida la excepción
  de política del sandbox de Homebrew en Linux. Es exactamente lo que ya hacemos en `hm_fail`
  (mensaje + siguiente acción); la novedad es hacerlo también en el instalador, que es donde el
  usuario está más perdido y donde todavía no tiene la herramienta para preguntar.
- **Banner y bloque de "next steps"** al final. Cosmético, pero es lo que cierra la sensación de
  que la instalación terminó.
- **Estructura del script**: bloques comentados por función (`setup_colors`, logging helpers,
  `detect_platform`, `check_prerequisites`, `detect_install_method`, `install_*`,
  `verify_installation`, `print_next_steps`, `main "$@"` al final). Nuestro `installer.sh` ya va en
  esa dirección; es la versión madura de la misma forma.

### GO-05 · Canales de instalación

**Esfuerzo**: S. **Es la pieza que habilita la beta 2.0 sin tocar a nadie más.**

gentle-ai tiene `--channel stable|beta|nightly` más variable de entorno `GENTLE_AI_CHANNEL`, con
validación contra lista blanca y `nightly` como alias de `beta`. Dos ficheros:
`scripts/install.sh` (resolución y validación en `main()`) e `internal/cli/channel.go` (el tipo
`InstallChannel` con `ResolveInstallChannel(flagValue)`, que cae a la variable de entorno cuando
la opción viene vacía).

En nuestro modelo el canal mapea limpiamente, porque *ya* tenemos el mecanismo: **canal → tag →
`git checkout` + binario de ese tag**. `HM_CHANNEL=beta curl … | bash` instala el prerelease de
2.0.0 (`.goreleaser.yaml:63` ya tiene `prerelease: auto`), mientras el resto del departamento
sigue en estable sin enterarse. Y `hm switch --list` / `--stable` ya son la mitad del trabajo.

Detalle a robar: la resolución del canal vive **en Go y en el script**, con la misma lista blanca.
El script lo necesita porque decide qué descargar antes de que exista binario; el Go lo necesita
para que `hm update` sepa en qué canal está. Es duplicación consciente y anotada.

### GO-06 · Instalar sin `sudo` por defecto

**Esfuerzo**: S. Parte de GO-04.

Hoy `create_link_to_command()` hace `sudo mkdir -p /usr/local/bin` y `sudo ln -s`
incondicionalmente. Su orden es: `/usr/local/bin` si existe **y es escribible**, si no
`~/.local/bin`, y `sudo` sólo como último recurso avisando; más `--dir` para forzar destino. En
portátiles corporativos con MDM eso es fricción evitable, y un `sudo` en un `curl | bash` es
justo lo que hace que alguien no lo ejecute.

### GO-07 · Dependencias mínimas del instalador

**Esfuerzo**: S.

Su script pide `curl` y `git`, y nada más. El nuestro exige `jq`, y si no está **instala Homebrew**
(`installer.sh:17-60`) o corre `sudo apt-get install`. Un instalador que instala Homebrew es un
instalador que puede tardar veinte minutos y fallar por causas que no son suyas.

En 2.0 `jq` es cada vez menos necesario: la capa Go no lo usa. Merece la pena comprobar si puede
pasar a ser requisito **del camino shell únicamente**, comprobado en `hm doctor` y no en la
instalación.

### GO-08 · Documentar el acoplamiento con `name_template`

**Esfuerzo**: S (es un comentario).

Copian el `name_template` del `.goreleaser.yaml` **dentro del script**, en un bloque de comentario
justo encima de la función que construye el nombre del archivo, con ejemplos resueltos. Es la clase
de acoplamiento que se rompe en silencio seis meses después, cuando alguien toca el template y el
instalador empieza a dar 404.

Nosotros tenemos exactamente el mismo acoplamiento —`installer.sh:101` construye
`hm_${system}_${architecture}` contra `.goreleaser.yaml:53` `name_template: "hm_{{ .Os }}_{{ .Arch }}"`—
y sin nota en ninguno de los dos lados.

### GO-09 · Compatibilidad con bash 3.2

**Esfuerzo**: 0 (es una regla a respetar).

macOS sigue trayendo bash 3.2, y `curl | bash` usa **ese**. Tienen un comentario explícito en
`install_go()` evitando `${var,,}` porque revienta con `bad substitution`, y usan
`tr '[:upper:]' '[:lower:]'`. Nuestro `installer.sh` usa `IFS='.' read -ra` y `[[ ]]`, que sí
funcionan en 3.2, pero es una trampa que hay que tener anotada al reescribir: nada de arrays
asociativos, `${var^^}`, `${var,,}` ni `mapfile` en el instalador.

---

## 4. GoReleaser

### GO-10 · Firma ad-hoc del binario darwin

**Esfuerzo**: S. **Alto valor por línea.** De `engram/.goreleaser.yaml`:

```yaml
hooks:
  post:
    - cmd: sh -c 'if [ "{{ .Os }}" = "darwin" ] && command -v codesign >/dev/null 2>&1; then codesign --force --sign - "{{ .Path }}"; fi'
```

Un binario darwin sin firmar, descargado por `curl`, puede ser rechazado por Gatekeeper; en arm64
un binario sin ninguna firma se rechaza directamente. Nosotros **ya** construimos darwin en un
runner macOS (`.github/workflows/release.yml:18`, y con motivo: cgo para FSEvents de Compose), así
que `codesign` está disponible en el runner. Es una línea.

Complementario, en el instalador: `xattr -d com.apple.quarantine` sobre el binario descargado.

### GO-11 · `-trimpath`

**Esfuerzo**: S. gentle-ai pone `flags: [-trimpath]` en sus builds; nosotros no. Quita las rutas
absolutas de la máquina de compilación del binario. Gratis.

---

## 5. CI y release

### GO-12 · Separar `preflight` de `release`, con permisos distintos

**Esfuerzo**: M. Es la forma del workflow de release de gentle-ai, y merece la pena entera.

Nuestro `.github/workflows/release.yml:7-8` da `permissions: contents: write` a **todo** el
workflow, incluidos los pasos de build. El suyo:

- `permissions: contents: read` a nivel de workflow.
- Job `preflight` (read-only) que compila todos los objetivos, corre tests, `vet` y formato, y
  **resuelve el plan de release completo con `--snapshot --skip=sign,publish`** para validarlo sin
  publicar nada.
- Job `release` con `permissions: contents: write`, que es el único que ve el token, y se lo pasa
  sólo a GoReleaser: ningún paso de `run` lo recibe, y el checkout va con
  `persist-credentials: false`.
- El `preflight` **exige que CI esté verde en ese commit exacto** antes de firmar bytes
  (`scripts/require-ci-success.sh`, que consulta la conclusión del workflow de CI vía API con
  `permissions: actions: read`). Release y CI son workflows independientes sobre el mismo commit;
  sin esa consulta, un tag sobre un commit rojo publica igual.

Nuestro `release.yml:27-33` ya compila las cuatro combinaciones antes de invocar GoReleaser: **eso
es el preflight en embrión**. Falta separarlo en job propio, quitarle el permiso de escritura y
añadirle la consulta a CI.

### GO-13 · Higiene del workflow

**Esfuerzo**: S.

- **Actions pineadas a SHA**, no a `@v4`/`@v5`/`@v6`. Ellos:
  `actions/checkout@93cb6efe18208431cddfb8368fd83d5badbf9bfd # v5.0.1`. Nosotros usamos etiquetas
  móviles en `release.yml:21,25,36`.
- **`concurrency: { group: release-${{ github.ref }}, cancel-in-progress: false }`**. No lo
  tenemos: dos pushes de tag cercanos pueden solaparse.
- **`fetch-tags: true`** en el checkout, con el comentario que ellos dejaron contando que se les
  rompió por no tenerlo (`actions/checkout` usa `--no-tags` por defecto, así que un preflight que
  valide el objeto de tag muere en un tag que **sí** está anotado).
- **Filtro de tags** para lo que no debe publicar. Ellos excluyen prereleases (`- "!v*-*"`) porque
  su preflight sólo acepta semver estable. Nosotros queremos lo contrario —`v2.0.0-beta.1` **sí**
  debe publicar, como prerelease— y `prerelease: auto` ya lo resuelve. Anotado para no copiarlo
  por inercia.

### GO-14 · El chequeo de formato como programa Go

**Esfuerzo**: S. Tienen `internal/gofmtcheck`, invocado como `go run ./internal/gofmtcheck`, en vez
de un paso de shell. Ventaja real: se ejecuta idéntico en local y en CI, sin depender de que el
`gofmt` del `PATH` sea el mismo.

Nuestro `.github/workflows/go.yml` inlinea `gofmt -l ./cmd ./internal`, que funciona pero (a) sólo
existe en CI y (b) `gofmt` no comprueba el orden de imports que sí comprueba `goimports`.

### GO-15 · Autotest del binario en CI

**Esfuerzo**: S. `engram/.github/workflows/ci.yml` hace
`go build -o engram-selftest ./cmd/engram && ./engram-selftest test --quick`: compila y **ejecuta**
el binario contra un subcomando de autodiagnóstico. Es una prueba de humo del punto de entrada
real, no de los paquetes.

Nuestro `doctor` es casi eso. Un `hm doctor --self-test` que no necesite Docker sería exactamente
la misma pieza, y encaja en CI donde hoy sólo compilamos.

**Donde vamos por delante**: nuestro `go.yml` corre `go test ./... -race`; gentle-ai corre
`go test ./...` sin race. Y nuestra suite de bash en CI no tiene equivalente en ninguno de los dos.

---

## 6. Versión y autochequeo

### GO-16 · Resolución de versión con fallback a `debug.BuildInfo`

**Esfuerzo**: S. `gentle-ai/internal/app/version.go` es un fichero de 30 líneas que hace bien una
cosa: ldflags → `BuildInfo.Main.Version` → `"dev"`, con el lector de `BuildInfo` en una `var` de
paquete para poder falsearlo en tests.

Nuestro `internal/cli/run.go` (`goVersion`) lee el ldflag `Version` y `vcs.revision`, pero **no cae
a `Main.Version`**, así que un binario construido con `go install` desde un tag reporta `dev`
teniendo versión. Tres líneas.

### GO-17 · Chequeo honesto de "¿hay versión nueva?" → a `hm doctor`

**Esfuerzo**: M. **El más aplicable de todo el análisis, y la contrapartida natural de GO-01.**

`engram/internal/version/check.go` (170 líneas, con su test) consulta la release de GitHub y
devuelve un `CheckResult` con tres estados: `up_to_date`, `update_available`, `check_failed`. Lo
que hace bien, y que es la razón de copiarlo:

- **No confunde "estás al día" con "no pude comprobarlo"**. Cada fallo tiene su mensaje concreto:
  versión actual desconocida, build de desarrollo que no mapea a release, timeout de GitHub, HTTP
  no-200. Nada de degradar en silencio a "todo bien".
- **Timeout de 2 s** con `context.WithTimeout`, y `errors.Is(err, context.DeadlineExceeded)`
  distinguido del resto.
- **`GH_TOKEN`/`GITHUB_TOKEN` opcional** para el rate limit, y cuando la API devuelve 401/403 el
  mensaje **dice** que se pueden definir esas variables.
- Instrucciones de actualización **según plataforma** (`runtime.GOOS`).
- Comparación de semver propia y minúscula (`splitVersion` → `[3]int`), sin dependencia.

Aplicado a nosotros: un chequeo en `hm doctor` de **versión del binario ↔ versión del checkout ↔
última release**. Es el diagnóstico que hoy no existe para GO-01: mientras GO-01 impide que las dos
mitades se separen, GO-17 lo detecta cuando ya pasó.

Añadido de gentle-ai, si el chequeo se pone en cada invocación y no sólo en `doctor`:
`internal/update/cooldown.go` cachea el resultado con un periodo de gracia, para no pegarle a la
API de GitHub en cada `hm`.

### GO-18 · `InstallMethod` como enum tipado con estrategia por método

**Esfuerzo**: S (la idea; el modelado completo no aplica).

`gentle-ai/internal/update/types.go` modela que **actualizar depende de cómo instalaste**:
`InstallMethod` con constantes (`brew`, `go-install`, `binary`, `script`, `opencode-plugin`) y un
ejecutor que elige estrategia por método. Y `UpdateStatus` con siete estados, incluidos
`version-unknown`, `check-failed` y `dev-build` — otra vez, el fallo no se disfraza de éxito.

Nosotros tenemos un solo método de instalación pero **dos mitades** (checkout + binario), que es el
mismo problema con otra forma. Lo robable es la disciplina: `update` y `switch` no improvisan cada
uno su camino, comparten la función de GO-01.

---

## 7. Arquitectura Go: veredicto y lo poco a adoptar

**No seguir su ejemplo.** Datos, no impresiones:

| | gentle-ai | engram | hm |
|---|---|---|---|
| LOC Go | 370.356 | 138.588 | **7.027** |
| Fichero mayor | `internal/cli/review_facade.go` **143,8 KB** | `internal/store/store.go` **9.078 líneas** | `internal/app/checks.go` 615 líneas |
| `main.go` | 20 líneas ✅ | **3.264 líneas** ❌ | 15 líneas ✅ |
| Capa de dominio | no | no | `internal/core` |
| Puertos / interfaces | no | no | `internal/core/ports` |
| Adaptadores aislados | no | no | `internal/adapters/*` |
| Test mayor | `sync_test.go` **202,5 KB** | `store_test.go` 12.942 líneas | `doctor_test.go` 306 líneas |

Nuestro hexagonal (`core` → `ports` ← `app` ← `adapters`/`cli`) es de libro, y los dos repos son
ejemplos de lo que pasa cuando no lo tienes: `internal/cli` de gentle-ai empezó siendo un
`switch` en `run.go` y hoy es un god-package de 400 ficheros.

Dicho eso, hay cuatro cosas suyas que sí valen:

### GO-19 · Ninguno de los dos usa Cobra — y nuestro `switch` va camino de su `run.go`

**Esfuerzo**: M.

Primero, lo tranquilizador: **ni gentle-ai ni engram usan Cobra**. Los dos parsean a mano. Valida
nuestra decisión: no es una carencia, es la misma elección de dos proyectos Go maduros. (Cobra
aparece en nuestro `go.mod` sólo como dependencia indirecta del SDK de Docker.)

Ahora la advertencia. `internal/cli/run.go` es hoy un `switch` con
`jsonOutput, rest := wantsJSON(args[1:], stdout)` repetido en cada rama. Con 10 comandos se lee
perfectamente. Con 40 es **exactamente** cómo nació el `run.go` de 106 KB de gentle-ai: el mismo
`switch`, seis años de comandos.

Propuesta: una tabla de comandos como fuente única.

```go
type command struct {
    run                 func(args []string, stdout, stderr io.Writer, jsonOutput bool) int
    consumesGlobalFlags bool  // `exec` no: `hm exec grep --json` le pregunta --json a grep
    batch               int   // la tanda de MIGRATION.md
}

var commands = map[string]command{ … }
```

Lo que se gana además del router plano: hoy la información de los comandos vive **duplicada** en
`data/command_descriptions.json`, en el `switch` de `run.go`, en la tabla de
[MIGRATION.md](../../MIGRATION.md) y en `generate_completion.sh`. Con la tabla en Go, el dispatch,
`hm list`, la ayuda y las completions salen de un sitio, y `tests/unit/migration_status_test.sh`
—que ya comprueba que la tabla de MIGRATION.md no miente— tiene una fuente real contra la que
comparar en vez de un grep del árbol.

### GO-20 · `var` de paquete para testabilidad puntual

**Esfuerzo**: S.

`var execCommand = exec.Command`, `var lookPath = exec.LookPath`, `var buildInfoReader = debug.ReadBuildInfo`,
`var osStat = os.Stat`. Para una llamada aislada al sistema es más ligero que declarar un puerto.

Nuestro `ports.go` está bien como está: los puertos son para lo que el **dominio** necesita, y esa
es la razón de que `doctor` sea testeable sin Docker. Esto es para lo otro: la lectura de
`BuildInfo` en `internal/cli`, o un `exec` puntual en un adaptador, donde una interfaz con un
método y un fake es más ceremonia que valor.

### GO-21 · Ficheros por plataforma en vez de `runtime.GOOS` disperso

**Esfuerzo**: S.

Ellos separan por sufijo: `compatibility_writer_unix.go` / `_unsupported.go`,
`review_facade_input_unix.go` / `_windows.go` / `_other.go`,
`compatibility_transaction_nonwindows.go`.

Nosotros ya lo hacemos bien en `internal/adapters/machine/memory_{darwin,linux}.go`. Pero en
`internal/cli` hay seis `runtime.GOOS == "darwin"` dispersos:

- `internal/cli/describe.go:137,141`
- `internal/cli/project.go:71,76`
- `internal/cli/php.go:101` y su test condicionado en `php_test.go:16`

`project.go:76` es un `map[bool]string{true: "mac", false: "linux"}[runtime.GOOS == "darwin"]` para
elegir el overlay de Compose. Eso pide un `overlay_darwin.go` / `overlay_linux.go`: desaparece el
condicional, desaparece el `map[bool]` y desaparece el test que sólo corre en una plataforma
(`php_test.go:16` hace `t.Skip` fuera de darwin, así que hoy la mitad de esa lógica no se prueba
nunca en CI, que corre en `ubuntu-latest`).

### GO-22 · Enums tipados de estado en vez de booleanos

**Esfuerzo**: S. Ya lo hacemos en `core.Topology`; la observación es que lo generalicemos.

Los dos definen estados como `type CheckStatus string` con constantes, no como `bool` ni como
`string` libre. Efecto en un CLI con `--json`: el estado serializa legible, y añadir un cuarto
estado obliga a revisar los sitios que lo consumen. En `doctor`, `describe` y `list` —cuyo `--json`
van a leer agentes— es la diferencia entre un contrato y una cadena.

---

## 8. Contrato `--json`: hacerlo ahora, no en la fase 6

### GO-23 · Golden fixtures + schema por comando con `--json`

**Esfuerzo**: M. **Es lo único de esta lista que es mucho más barato ahora que después.**

gentle-ai congela su contrato de integración: `contracts/review-integration/v1/` y `/v2/` con
`schemas/*.schema.json`, `fixtures/*.fixture.json` y un `FREEZE.md`; se validan en CI, se
**publican como artefacto de la release** (van dentro del `archives:` de GoReleaser) y hay un
comando propio que verifica el bundle (`go run ./internal/providercontractbundlecmd verify`).
Versionan el contrato con su propio semver, independiente del de la herramienta
(`contracts/review-provider-contract/CONTRACT_SEMVER`).

Nosotros vamos a exponer `--json` a agentes: es la fase 6 de [MIGRATION.md](../../MIGRATION.md), y
el [README de research](README.md) ya identificó que el contrato JSON tiene tres consumidores —CLI,
agentes vía MCP y dashboard— y "se paga una vez y se cobra tres".

**Hazlo con 10 comandos portados, no con 64.** Un `testdata/golden/<comando>.json` por comando con
`--json`, más su schema, gateado en CI. Es la misma propiedad que hoy verificamos byte a byte
contra bash en `tests/integration/go_passthrough_test.sh` —pero que **sobrevive a la desaparición
del bash**, que es cuando dejaremos de tener con qué comparar. Hoy el oráculo es la
implementación shell; el día que se borre, sin fixtures no hay oráculo.

No hace falta copiarles el versionado independiente del contrato ni la publicación como artefacto
de release. Eso viene después, si algún día `hm` es dependencia de algo de fuera.

---

## 9. Guardas de la migración

### GO-24 · Ratchet de deadcode

**Esfuerzo**: M. **Es el patrón ideal para un strangler**, y es primo de algo que ya inventamos.

gentle-ai tiene `.deadcode-baseline.txt` (16,6 KB) más `scripts/deadcode-ratchet.sh`: un fichero
con el código muerto **conocido y aceptado**, y un paso de CI que falla si aparece código muerto
nuevo. No exige limpiar lo viejo; prohíbe añadir.

Durante una migración por estrangulamiento se acumulan exactamente dos cosas: shell que ya no
llama nadie porque su comando se portó, y Go a medio portar que quedó sin conectar. El ratchet
impide que eso crezca sin bloquear el avance.

Y es la misma idea que `tests/unit/migration_status_test.sh`, que ya comprueba que la tabla de
MIGRATION.md no miente. Ese test es, francamente, mejor idea que la mitad de lo que tienen ellos:
merece la pena verlo como el primero de una familia de guardas, no como un test suelto.

---

## 10. Orden sugerido

No hay urgencia por fecha. El orden sale de dependencias y de coste creciente:

**Ahora, porque son fallos presentes o porque se abaratan si se hacen ya**

1. **GO-01** — el binario sigue al checkout. Único con riesgo real hoy, y crece con cada comando
   portado. Toca `installer.sh`, `update.sh`, `switch.sh`; extiende REL-02 y REL-03.
2. **GO-23** — golden fixtures del `--json`. Con 10 comandos son 10 ficheros; con 64 es un
   proyecto. Y es el oráculo que sustituye al bash cuando el bash se vaya.
3. **GO-16**, **GO-11**, **GO-10** — versión con `BuildInfo`, `-trimpath`, `codesign` ad-hoc. Las
   tres son de una a tres líneas.

**Cuando se toque el área que les corresponde**

4. **GO-17** — chequeo de versión en `doctor`. Va con el siguiente cambio en `doctor`.
5. **GO-19** — tabla de comandos. El momento natural es cuando el `switch` pase de ~20 ramas, o
   cuando haya que generar completions desde Go.
6. **GO-21** — ficheros por plataforma en `internal/cli`. Va con el siguiente cambio en
   `describe`/`project`/`php`.
7. **GO-12**, **GO-13**, **GO-14**, **GO-15** — endurecer release y CI. Antes de que exista un tag
   2.0 que alguien de fuera del equipo instale.

**Cuando se prepare la beta de verdad**

8. **GO-04** (que absorbe GO-02 y GO-03) + **GO-05**, **GO-06**, **GO-07**, **GO-08**, **GO-09** —
   el instalador nuevo con canales. Depende de GO-01, y no tiene sentido pulirlo antes de saber
   qué comandos entran en la beta.

**Cuando compense, o nunca**

9. **GO-24** — ratchet de deadcode. Útil desde ya, pero es proceso: sólo si el shell muerto empieza
   a acumularse de verdad.
10. **GO-18**, **GO-20**, **GO-22** — disciplinas, no tareas. Se aplican al escribir código nuevo.

---

## 11. Tabla resumen

| ID | Qué | Área | Esfuerzo | Estado |
|---|---|---|---|---|
| GO-01 | El binario debe seguir al checkout (`update`/`switch`/instalador) | Release | M | **el importante** |
| GO-02 | `installer.sh` idempotente | Instalación | S | absorbido por GO-04 |
| GO-03 | Verificar `checksums.txt` (fail closed) | Instalación | S | absorbido por GO-04 |
| GO-04 | `scripts/install.sh` reescrito | Instalación | M | depende de GO-01 |
| GO-05 | Canales `stable`/`beta` (flag + env, en script y en Go) | Instalación | S | habilita la beta |
| GO-06 | Instalar sin `sudo` por defecto, `--dir` | Instalación | S | parte de GO-04 |
| GO-07 | Instalador sin `jq`/Homebrew | Instalación | S | |
| GO-08 | Documentar acoplamiento con `name_template` | Release | S | un comentario |
| GO-09 | Respetar bash 3.2 en el instalador | Instalación | 0 | regla |
| GO-10 | `codesign` ad-hoc del binario darwin | Release | S | una línea |
| GO-11 | `-trimpath` | Release | S | una línea |
| GO-12 | `preflight`/`release` con permisos separados + CI verde exigido | Release | M | |
| GO-13 | Actions pineadas, `concurrency`, `fetch-tags` | Release | S | |
| GO-14 | Chequeo de formato como programa Go | CLI | S | |
| GO-15 | Autotest del binario en CI (`doctor --self-test`) | CLI | S | |
| GO-16 | Versión con fallback a `debug.BuildInfo` | CLI | S | tres líneas |
| GO-17 | Chequeo honesto de versión nueva → `doctor` | Release | M | contrapartida de GO-01 |
| GO-18 | `InstallMethod`/`UpdateStatus` tipados, estrategia por método | Release | S | disciplina |
| GO-19 | Tabla de comandos en vez de `switch` creciente | CLI | M | |
| GO-20 | `var` de paquete para testabilidad puntual | CLI | S | disciplina |
| GO-21 | Ficheros por plataforma en vez de `runtime.GOOS` disperso | CLI | S | |
| GO-22 | Enums tipados de estado | CLI | S | disciplina |
| GO-23 | Golden fixtures + schema del `--json` | CLI | M | **abaratado ahora** |
| GO-24 | Ratchet de deadcode | CLI | M | proceso |

**Descartado con motivo** (§1): organización de paquetes, Homebrew tap, firma minisign, ratchets de
prompts, filtro de tags que excluye prereleases.
