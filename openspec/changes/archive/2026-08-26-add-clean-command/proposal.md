# Recoger lo que dejaron los proyectos que ya no existen

## Por qué

Una máquina que lleva tiempo levantando proyectos acumula contenedores y volúmenes de entornos
que ya no existen: la carpeta se borró, la rama se cerró, el cliente terminó. Nadie los reclama y
nadie los borra, porque hacerlo a mano exige averiguar cuál es de qué.

En esta misma máquina hay **124 volúmenes**, y proyectos con volúmenes pero sin un solo contenedor
desde hace tiempo.

La alternativa que existe hoy es `docker system prune`, y es exactamente la herramienta
equivocada: no distingue lo nuestro de lo ajeno, ni un proyecto muerto de uno parado. Durante el
desarrollo de esta versión se ejecutó un `docker volume prune` sin comprobar antes su alcance; salió
bien por cómo se comporta Docker 27, no por prudencia.

Hace falta un recogedor que sepa de qué está hablando.

## Qué cambia

- **`hm clean`** muestra lo que se puede recoger y **no borra nada**. Ese es su comportamiento por
  defecto, no una opción.
- Con `--force` borra, después de enumerar y preguntar.
- Sólo toca lo que **puede demostrar** que es un entorno de Dockergento abandonado: contenedores con
  etiqueta `hm.*` cuyo directorio de origen ya no existe, y los volúmenes de esos mismos proyectos.
- **Nunca toca un proyecto cuyo directorio sigue estando**, esté parado o en marcha.
- Informa de lo que ve pero no le pertenece —imágenes sueltas, caché de construcción— y dice con qué
  comando de Docker se limpia, sin ejecutarlo.

## Qué no cambia

- No se ejecuta ningún `prune`. Ni con `--force`.
- Las copias de base de datos no se tocan: son de `hm db clear`, y son lo último que uno querría
  perder en una limpieza.

## Un límite que conviene decir en voz alta

Los volúmenes **no llevan etiquetas `hm.*`**: sólo las que pone Compose. Así que de un proyecto sin
ningún contenedor no se puede demostrar que fuera nuestro, y `hm clean` no lo tocará. Se listan
aparte, como «no se puede atribuir», con su nombre, para que la decisión la tome una persona.

Preferimos dejar basura a borrar algo de alguien.

## Cómo se sabrá que funciona

- Sin opciones no borra nada, nunca.
- Un proyecto cuyo directorio existe no aparece jamás entre lo recogible.
- Un proyecto cuyo directorio no existe aparece con sus contenedores y sus volúmenes.
- Los volúmenes que no se pueden atribuir se listan aparte y no se borran ni con `--force`.
