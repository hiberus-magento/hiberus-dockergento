## Context

Este es el comando más peligroso del roadmap: su trabajo es borrar. Todo el diseño consiste en
acotar qué puede llegar a tocar, y el resto es presentación.

Dos hechos comprobados en esta máquina antes de decidir nada:

- **Los volúmenes no llevan etiquetas `hm.*`.** Un `docker volume inspect` de un volumen de proyecto
  devuelve sólo `com.docker.compose.*`. Las etiquetas nuestras están en los contenedores, porque es
  ahí donde la plantilla las pone.
- **Calcular el tamaño de los volúmenes cuesta 25 segundos** (`docker system df -v`, 124 volúmenes).
  No es algo que pueda hacerse en el camino normal de un comando.

El primero decide el alcance; el segundo, la presentación.

## Goals / Non-Goals

**Goals**
- Recuperar espacio de entornos que ya no existen, sin que nadie tenga que averiguar cuál es cuál.
- Que sea imposible perder algo vivo con este comando.

**Non-Goals**
- No es `docker system prune` con otro nombre. No toca nada que no pueda atribuir.
- No borra copias de base de datos. Son de `hm db clear` y son lo último que se querría perder.
- No borra imágenes: son compartidas entre proyectos y volver a descargarlas es caro.

## Decisions

### 1. No borrar es el comportamiento, no una opción

El backlog pedía `--dry-run`. Se invierte: **`hm clean` no borra**, y borrar es `--force`. Un
`--dry-run` que hay que acordarse de escribir protege a quien ya tiene cuidado; el peligro está en
quien escribe el comando sin leer la documentación, y para ese la única defensa es que la forma
corta sea la inofensiva.

### 2. Sólo se toca lo demostrablemente abandonado

Un entorno es recogible cuando cumple **las dos** condiciones:

1. Tiene contenedores con etiqueta `hm.project`, es decir, lo creamos nosotros.
2. Su `hm.root` **ya no existe** en el disco.

Un proyecto parado cuyo directorio sigue ahí no es basura: es un proyecto parado. Es la
distinción que `docker system prune` no sabe hacer, y la razón de que este comando exista.

Para los entornos anteriores a las etiquetas —los que sólo se reconocen por tener un servicio
`phpfpm`— no hay `hm.root` que comprobar, así que no se recogen. Se listan como no atribuibles.

### 3. Los volúmenes se atribuyen por sus contenedores, no por sí mismos

Como los volúmenes no llevan etiquetas nuestras, la única forma honesta de saber que un volumen es
de un entorno nuestro es que **su proyecto tenga contenedores que sí las lleven**. Los volúmenes de
un proyecto del que ya no queda ni un contenedor no se pueden atribuir: podrían ser de un stack de
Compose que alguien escribió a mano.

Esos se listan aparte, con su nombre, y no se borran ni con `--force`. Preferimos dejar basura a
borrar algo de alguien.

Es una limitación real y tiene arreglo: si en el futuro la plantilla etiqueta también los
volúmenes, este comando podrá atribuirlos y recogerlos. Hasta entonces, se dice.

### 4. El tamaño se calcula sólo cuando se va a borrar

Veinticinco segundos es demasiado para un comando que se ejecuta para mirar. Sin `--force` se
informa de **cuántos** recursos hay; con `--force`, antes de preguntar, se calcula el espacio que se
va a liberar, porque ahí sí compensa esperar: es el dato que decide la respuesta.

### 5. Lo que no es nuestro se informa, no se toca

Imágenes sueltas y caché de construcción ocupan y no son de nadie en particular. Se dice cuánto
ocupan y con qué comando de Docker se limpian, **sin ejecutarlo**. Que la herramienta ejecute un
`prune` por su cuenta es precisamente el error que este comando viene a no cometer.

### 6. Con `--force` se pregunta igual

`--force` significa «quiero borrar», no «no me preguntes». La lista de lo que se va a borrar sigue
apareciendo y sigue habiendo una confirmación, que se salta con `--yes` como en todo lo demás.

## Risks / Trade-offs

- **Deja basura sin recoger**: los volúmenes no atribuibles se quedan. Es el precio de no borrar de
  nadie, y se paga a gusto.
- **`hm.root` puede existir y estar vacío** —un directorio que quedó tras borrar el contenido—, y
  entonces el entorno no se considera abandonado. Falso negativo, que es el lado bueno del error.
- **Un proyecto en un disco desmontado** parece abandonado. Se mitiga: se enumera antes de borrar y
  hay confirmación; quien vea un nombre conocido, cancela.

## Migration Plan

Ninguna: comando nuevo que por defecto no hace nada destructivo.

## Open Questions

Si conviene etiquetar los volúmenes en la plantilla para poder atribuirlos. Es un cambio de una
línea, pero sólo afecta a los volúmenes creados después, así que no resuelve el caso que hoy duele.
Queda anotado, no decidido.
