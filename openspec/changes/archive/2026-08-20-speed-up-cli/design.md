## Context

`hm` es Bash, así que su rendimiento es, casi por completo, **cuántos procesos lanza**. Un
`jq` cuesta 36 ms; una llamada a Docker, entre 70 y 190 ms. No hay algoritmos que optimizar:
hay procesos que no lanzar.

El reparto medido del gasto:

```
hm --help    5,7 s   ██████████████████████████  143 jq (5,1 s) + arranque (0,3 s)
hm doctor    4,0 s   ██████████████████          12 checks secuenciales
hm describe  1,3 s   ██████                      3 llamadas a docker + arranque
hm --version 0,3 s   █                           suelo: bash + 2 jq + 3 git
```

El suelo de 0,30 s es el arranque mínimo: iniciar bash, cargar seis ficheros de código,
leer las propiedades con `jq` y resolver la raíz del proyecto con `git`.

## Goals / Non-Goals

**Goals:**
- `hm --help` por debajo de 0,5 s: es la primera impresión de la herramienta.
- Que ninguna invocación pague por trabajo que no va a usar.
- Que la mejora quede vigilada por una prueba, no por la memoria de quien la hizo.

**Non-Goals:**
- Bajar el suelo de 0,30 s a base de trucos: no merece el riesgo.
- Cambiar de lenguaje.
- Cachés que puedan servir datos obsoletos sin forma de detectarlo.

## Decisions

### 1. Una consulta a `jq`, un bucle en Bash

`print_commands_info` hace hoy, por cada comando: extraer su objeto, extraer la
descripción y extraer la marca `mac`. Son tres procesos por comando.

En su lugar, una única invocación produce las líneas ya formateadas
(`nombre<US>descripción<US>mac`) y Bash sólo itera e imprime. De 143 procesos a 1.

Alternativa descartada: cachear el resultado del listado en disco. Añade invalidación y
resuelve un problema que desaparece solo al no lanzar 143 procesos.

### 2. Trabajo perezoso, no trabajo anticipado

Tres cosas se calculan hoy siempre y se usan casi nunca:

| Trabajo | Coste | Quién lo necesita de verdad |
|---|---|---|
| `docker compose version` | 188 ms | Sólo quien compara versiones (generación de plantillas, `doctor`) |
| Versión de Magento desde `composer.lock` | 77 ms | Sólo las etiquetas, al crear contenedores |
| `git describe` para la versión de `hm` | 56 ms | Sólo las etiquetas y `--version` |

Se convierten en funciones que calculan **la primera vez que se las llama** y memorizan el
resultado en una variable del shell. El patrón ya existe en el proyecto
(`compose_config_json`), con una lección aprendida: **memorizar sólo funciona si la
asignación ocurre en el shell del llamante**, nunca dentro de `$( )`.

Consecuencia importante: las etiquetas `hm.*` se interpolan al arrancar contenedores, así
que las variables deben estar exportadas **antes** de cualquier `docker compose up`. El
cálculo perezoso se fuerza en los comandos que crean contenedores, no en todos.

### 3. Validación de Compose con caché, fuera del repositorio

`docker compose config -q` se ejecuta en casi todos los comandos para comprobar que la
configuración es válida. Medido dos veces en la misma máquina: **72 ms en caliente y 325 ms
en frío**, es decir hasta un 20 % de un comando típico. Merece una caché.

Lo que no merece es guardarla dentro del proyecto. `config/docker/` **está versionado** en
los proyectos reales, así que un fichero de caché ahí aparecería en cada `git status`,
acabaría colándose en algún commit, y obligaría a añadir una entrada al `.gitignore` de cada
uno de los repositorios del departamento.

La caché vive por tanto en `~/.hm/cache/<huella de la ruta del proyecto>`: nada que
versionar, nada que excluir, ningún repositorio que tocar. El coste es dejar ficheros
minúsculos en el `HOME`, que `hm clean` puede barrer.

Contenido: la marca de tiempo más reciente de los ficheros de Compose en el momento de la
última validación correcta. Si coincide, se salta la validación.

Cómo se acotan los riesgos: se invalida por fecha y no por contenido, lo que es más barato y
falla por el lado seguro —revalida de más, nunca de menos—; una caché ilegible o corrupta se
trata como ausente; y `hm doctor` valida siempre, sin consultar la caché, para que exista
una forma de comprobar de verdad.

Alternativa descartada: `config/docker/.validated` con entrada en el `.gitignore` de cada
proyecto. Mismo beneficio, pero reparte fricción entre N repositorios y basta con que a uno
se le olvide para que la caché acabe versionada.

### 4. `doctor` en paralelo

Doce comprobaciones independientes, cada una en su proceso, hoy secuenciales. Se lanzan
todas a la vez escribiendo en ficheros temporales y se recogen en orden de nombre, para que
la salida sea **determinista** aunque terminen desordenadas.

El límite de tiempo por comprobación se mantiene: en paralelo protege igual, y ahora el
peor caso total es el de la comprobación más lenta, no la suma.

Alternativa descartada: reducir el número de comprobaciones. El valor de `doctor` está
justamente en la cobertura.

### 5. Presupuesto vigilado

Una prueba mide y falla si se excede:

| Ruta | Presupuesto |
|---|---|
| `hm --help` | 500 ms |
| `hm --version` (suelo de arranque) | 400 ms |
| `hm doctor` | 2 s |

Los presupuestos son deliberadamente holgados respecto al objetivo, porque una prueba de
tiempos en máquinas distintas y con Docker de por medio es inherentemente ruidosa. La
prueba está en este mismo cambio: la misma llamada a `compose config -q` midió 72 ms en
caliente y 325 ms en frío. Un presupuesto debe detectar una regresión de un orden de
magnitud, no una fluctuación del cuádruple por caché de disco.

### 6. El suelo de arranque: fusionar las llamadas a git, y nada más

El suelo de 0,30 s que paga toda invocación se reparte así: arrancar bash 76 ms, cargar los
ficheros de código ~100-160 ms, **tres llamadas a `git rev-parse` 132 ms** y una a `jq` para
las propiedades 59 ms.

Las tres llamadas a git vienen de la resolución de worktree y se pueden hacer en una:

```
git rev-parse --path-format=absolute --git-common-dir --show-toplevel
```

Devuelve las dos rutas que la lógica compara, y en un directorio que no es un repositorio
**falla**, lo que hace innecesaria la pregunta previa `--is-inside-work-tree`: el fallo es
la respuesta. Comprobado en los tres escenarios: checkout principal, worktree y fuera de
git. Medido en aislamiento, **132 ms → 46 ms**.


**Corrección medida.** Estimado el ahorro en 86 ms a partir de esas llamadas aisladas, el
A/B sobre el comando real da **~14 ms, por debajo del ruido de medición (±60 ms)**: cada
llamada aislada pagaba un arranque en frío que en el flujo real no se paga, porque git ya
tiene sus cachés calientes.

Se mantiene el cambio —quita dos procesos, no altera ninguna decisión de la lógica y pasa
las 35 pruebas del resolvedor— pero **no se justifica por el tiempo**, sino por ser código
más simple; y el margen contará más en máquinas lentas y en agentes que hacen cientos de
llamadas. La lección para el resto del cambio: el coste de una operación medida en
aislamiento no es su coste dentro del flujo.


Se hace porque es mecánico —no cambia ninguna decisión de la lógica— y porque el fichero que
toca está cubierto por 9 pruebas unitarias y 26 aserciones de integración. Ese colchón es la
condición para tocarlo: es el mismo fichero cuyo error de subshell destruyó un entorno de
desarrollo.

**No se toca el coste de `source`**, que son los otros ~150 ms. Reducirlo obliga a cargar
ficheros de forma perezosa, y eso cambia *cuándo* está definida cada función: el tipo de
cambio sutil que rompe cosas lejanas y difíciles de atribuir. El beneficio para una persona
es imperceptible.

Y ahí está el criterio para justificar los 86 ms: **nadie los nota escribiendo un comando a
mano**. Los nota el TUI, que refresca la flota, y los notan los agentes, que hacen decenas de
llamadas seguidas. Se optimiza para esos dos consumidores, no para la percepción humana.

### 7. mac y linux

Sin diferencias de diseño. Los costes unitarios sí difieren —Docker Desktop en macOS
atraviesa una VM— así que el presupuesto se fija con margen para la plataforma más lenta.

## Risks / Trade-offs

- **Una caché que sirve datos obsoletos** → invalidación por marca de tiempo, fichero no
  versionado, y `doctor` que nunca la consulta.
- **El cálculo perezoso llega tarde a las etiquetas** → los comandos que crean contenedores
  fuerzan el cálculo antes de invocar a Compose, y hay una prueba que comprueba que un
  contenedor recién creado lleva la versión de Magento en su etiqueta.
- **Salida no determinista al paralelizar `doctor`** → recogida ordenada por nombre de
  fichero, con una prueba que compara el orden con el de la ejecución secuencial.
- **Pruebas de tiempo intermitentes** → presupuestos holgados y medición del mejor de tres
  intentos, no de uno.
- **Optimizar lo que no importa** → todo cambio de esta lista sale de una medida, y el orden
  lo fija la medida: primero los 5,1 s del listado, después los 4 s de `doctor`, y sólo
  entonces los milisegundos del arranque.

## Migration Plan

Ninguna migración, y **ningún cambio en los proyectos**: la caché vive en `~/.hm/cache/`,
así que no hay `.gitignore` que tocar. Es una versión normal de `hm`.

Reversión: revertir el commit y, si se quiere, borrar `~/.hm/cache/`. No hay estado del que
dependa nada.

## Open Questions

Ninguna. La duda sobre el suelo de arranque quedó resuelta en la decisión 6: se fusionan las
llamadas a `git rev-parse` (~14 ms reales, no los 86 ms estimados) y se descarta
explícitamente el `source` perezoso.
