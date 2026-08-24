## Context

El nombre del proyecto es la identidad del entorno: da nombre a los contenedores, a la red y —lo
que de verdad importa— a los **volúmenes**. Cambiárselo a un entorno existente no es un cambio de
etiqueta: es dejar su base de datos colgada de un nombre que ya nadie consulta.

Eso convierte este cambio en un ejercicio de contención. La funcionalidad que se pide —que el
directorio decida cuando nadie ha decidido— es de diez líneas. El trabajo está en garantizar que
no toca a nadie que ya tenga un nombre puesto.

Hay además un desajuste anterior que este cambio cierra. Con la propiedad vacía, Compose deriva
el nombre del directorio y `hm` no, así que ambos operan con nombres distintos sobre los mismos
contenedores: las etiquetas `hm.*` salen vacías y las búsquedas por servicio fallan.

## Goals / Non-Goals

**Goals**
- Un único nombre resuelto, compartido por la CLI y por Compose, imposible de desincronizar.
- Que un directorio distinto sea un entorno distinto, sin configurar nada.
- Cero movimiento para los proyectos que ya existen.

**Non-Goals**
- No se renombra nada. No hay migración, ni asistente, ni conversión.
- No se habilita un entorno por worktree: eso es WT-02 y depende del proxy.
- No se toca `hm setup -f` sobre proyectos existentes: si el fichero ya tiene nombre, se respeta.

## Decisions

### 1. La regla de derivación se copia de Compose, medida y no supuesta

Reproducir «más o menos» la regla de Compose sería peor que no derivar nada: bastaría un
directorio con un punto o un acento para que la CLI y Compose discreparan justo en el momento de
borrar algo. La regla se comprobó ejecutando `docker compose config` sobre directorios reales:

| Directorio | Nombre de Compose |
|---|---|
| `UPPER` | `upper` |
| `con espacio` | `conespacio` |
| `punto.com` | `puntocom` |
| `acentúado` | `acentado` |
| `--weird--` | `weird--` |
| `___` | error: *project name must not be empty* |

Es decir: minúsculas, quedarse sólo con `[a-z0-9_-]` —los caracteres acentuados se **eliminan**,
no se transliteran— y recortar los `-` y `_` iniciales. Si no queda nada, no hay nombre.

Un test compara nuestra derivación con la de Compose sobre esa misma tabla, así que si Compose
cambia de criterio en una versión futura, lo sabremos por un fallo y no por un volumen perdido.

### 2. El nombre configurado gana siempre, y no se toca

La resolución es un `if` de una línea, y el orden importa más que el código:

1. `COMPOSE_PROJECT_NAME` de las propiedades del proyecto, si no está vacío.
2. Si no, el derivado del directorio del proyecto.

Nunca al revés, y nunca «derivar y comparar». Para todo el que tiene hoy la propiedad puesta
—que son todos los proyectos del departamento— este cambio es inerte por construcción.

### 3. El directorio del que se deriva es la raíz del proyecto, no `$PWD`

Desde un worktree la raíz es el checkout principal. Derivar de `$PWD` daría a ese worktree una
identidad propia, justo lo que los guardarraíles impiden crear. Se deriva de la raíz que ya
resuelve `hm_resolve_project_root`.

(Comprobado de paso: `hm` no funciona desde un subdirectorio del proyecto —falla con el código de
proyecto porque los ficheros de compose se resuelven contra `$PWD`—, ni antes ni después de este
cambio. Es una limitación anterior y queda fuera de este alcance.)

Como efecto secundario coherente: desde un worktree el nombre derivado es el del checkout
principal, que es el entorno sobre el que los guardarraíles dicen que se está operando.

### 4. `setup` guarda el nombre sólo cuando es una decisión

Hoy `setup` escribe siempre el nombre en el fichero versionado, aunque sea el que ya habría salido
del directorio. Guardar una decisión que no se ha tomado es lo que hace que un clon herede la
identidad del original.

A partir de ahora: si lo que se acepta coincide con el derivado, no se escribe la propiedad. Si es
distinto, se escribe, porque entonces sí es una decisión.

La pregunta no cambia y el valor por defecto tampoco, así que la experiencia de `hm setup` es la
misma. Lo que cambia es lo que queda escrito en un fichero que se comparte con el equipo.

### 5. Un directorio sin nombre válido se rechaza en la CLI

`___` o `...` producen un nombre vacío. Compose responde con *project name must not be empty*, que
no dice qué hacer. La CLI comprueba antes y falla con el código de proyecto, explicando las dos
salidas: renombrar el directorio o fijar `COMPOSE_PROJECT_NAME`.

### 6. Sin caché

La derivación son expansiones de parámetros de Bash sobre una cadena: cero procesos. Cachearla
costaría más que calcularla, y añadiría un sitio donde el nombre puede quedarse obsoleto.

## Risks / Trade-offs

- **Un proyecto sin nombre configurado cambia de identidad si se renombra su directorio.** Es
  inherente a derivar del directorio, y es lo que ya hace Compose hoy en ese caso: la CLI pasa a
  coincidir con esa realidad en lugar de contradecirla.
- **Dos directorios distintos pueden derivar el mismo nombre** (`Mi-Proyecto` y `mi_proyecto` no,
  pero `proyecto` y `proyecto.` sí). Es de nuevo el comportamiento de Compose; si molesta, la
  salida es fijar el nombre.
- **`setup` deja de escribir una propiedad que antes escribía.** Un script del equipo que lea
  `COMPOSE_PROJECT_NAME` de `properties.json` dando por hecho que existe encontrará el campo
  vacío. `hm describe` informa del nombre resuelto y es la fuente correcta para eso.

## Migration Plan

Ninguna. Los proyectos existentes tienen la propiedad puesta y quedan intactos. Quien quiera el
comportamiento nuevo en un proyecto existente vacía la propiedad a mano, sabiendo que eso es
renombrar el entorno y que sus volúmenes se quedan en el nombre anterior.

## Open Questions

Ninguna.
