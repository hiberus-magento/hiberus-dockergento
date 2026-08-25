## Context

La herramienta ya sabe volcar e importar: `hm mysqldump` y `hm mysql -i` llevan años ahí. Lo que
falta no es capacidad, es **gestión**: un sitio donde vivan las copias, un nombre por el que
llamarlas y una lista donde verlas. Sin eso, guardar antes de una operación de riesgo depende de
que a alguien se le ocurra, y por eso no ocurre.

Comprobado en la imagen de MariaDB del proyecto (12.3) antes de decidir nada: están
`mariadb-backup` y `mariabackup` para copia física, y `mariadb-dump` para la lógica. Las dos vías
eran técnicamente posibles.

## Goals / Non-Goals

**Goals**
- Guardar y volver, con nombre, sin pensar dónde.
- Que la copia sobreviva a la destrucción del entorno, que es cuando hace falta.
- Que restaurar deje la base de datos idéntica, no *parecida*.

**Non-Goals**
- No es una herramienta de copias de seguridad. Son copias locales de trabajo, sin cifrado, sin
  rotación y sin nada remoto.
- No sustituye a `hm mysqldump`, que sigue siendo la vía para exportar a una ruta concreta.
- No se toca el ciclo de vida de los contenedores: eso es DB-03.

## Decisions

### 1. Volcado lógico, no copia física

El backlog proponía `mariadb-backup`. Se descarta contra su propio criterio de aceptación —
*restaurar sin parar el proyecto* —, que la copia física no puede cumplir: restaurarla es
reemplazar el directorio de datos de un servidor que tiene que estar apagado mientras se hace.

| | Física (`mariadb-backup`) | Lógica (`mariadb-dump`) |
|---|---|---|
| Crear | En caliente | En caliente, con `--single-transaction` |
| Restaurar | **Exige parar el servidor** | Con el proyecto en marcha |
| Entre versiones de MariaDB | No | Sí |
| Tamaño | El del directorio de datos | Menor, y se comprime muy bien |
| Velocidad | Mayor | Menor |

Se elige la lógica. Es más lenta, y a cambio hace lo que se pedía.

### 2. Las copias viven fuera del proyecto

En `~/.hm/snapshots/<proyecto>/`, junto a la caché y por las mismas razones:

- **Ningún `.gitignore` que tocar.** `config/docker/` está versionado; una copia ahí acabaría en un
  commit, y son decenas de megas.
- **Sobreviven a `hm down -v`.** Guardar la copia dentro del entorno que se va a destruir sería
  guardarla en el sitio exactamente equivocado.
- El nombre del proyecto que agrupa las copias es el **nombre resuelto**, el mismo que usan las
  etiquetas y el inventario, así que dos proyectos distintos no se mezclan aunque estén en
  directorios parecidos.

### 3. Comprimidas siempre, sin opción

Un volcado de un Magento real ronda los cientos de megas y comprime a una fracción. `gzip -6` es
suficientemente rápido para no notarse frente al tiempo del volcado, y la alternativa —dejarlo
elegir— es una opción más que documentar para una decisión que tiene una respuesta correcta.

### 4. Restaurar vacía primero

Restaurar sobre una base de datos que ha seguido viviendo deja lo que había de más: una tabla
creada después de la copia sigue ahí, y el resultado no es la copia sino una mezcla. Se elimina el
esquema y se crea de nuevo antes de cargar, de modo que restaurar devuelve exactamente lo copiado.

### 5. Restaurar pregunta, salvo que se le diga que no

Es la única operación destructiva del comando y no tiene vuelta atrás. Pregunta antes, nombrando
la copia y el proyecto. Con `--yes` —la opción global que ya existe— no pregunta, para que sirva
en un guion.

Se pide confirmación escribiendo el nombre del proyecto, no un sí. Un `y` a ciegas es un reflejo;
teclear el nombre exige haber leído la frase.

### 6. Los metadatos van en el nombre del fichero, no en un índice

Nombre, fecha y tamaño salen del propio fichero (`<nombre>.sql.gz` y su `mtime`). Sin fichero de
índice: uno más que mantener sincronizado, y que se desincroniza en cuanto alguien borra una copia
a mano.

Lo que no cabe en el nombre —la versión de Magento con la que se hizo— se guarda como una línea de
comentario dentro del propio volcado, que es donde no se puede perder.

## Risks / Trade-offs

- **Restaurar deja la tienda inconsistente mientras dura.** El proyecto sigue en marcha, pero su
  base de datos se está reescribiendo. Es local; se documenta.
- **Una copia grande tarda.** Es el precio de la vía lógica, y la elegimos a sabiendas.
- **Las copias ocupan espacio y nadie las borra.** `hm db remove` existe, y CLI-08 (`hm clean`) las
  tendrá en cuenta.
- **`~/.hm/snapshots` no se comparte entre máquinas.** Es deliberado: son copias de trabajo, no un
  respaldo del equipo.

## Migration Plan

Ninguna: comando nuevo.

## Open Questions

Ninguna. Si aparece la necesidad de copias físicas —bases de datos donde el volcado lógico tarde
demasiado—, se puede añadir como una segunda estrategia, y entonces habrá que aceptar que
restaurar pare la base de datos.
