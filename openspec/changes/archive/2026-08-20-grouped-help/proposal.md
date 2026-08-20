## Why

`hm --help` presenta **45 comandos en una lista alfabética plana**, sin línea de uso, sin
ejemplos y sin ninguna pista de qué hace cada cosa ni de por dónde empezar. Quien llega
nuevo ve `ai-init` antes que `start`, y quien lleva dos años sigue sin saber que existe
`copy-from-container` hasta que alguien se lo dice.

Encima, ocho de las primeras líneas de la pantalla son un logo, así que lo que el usuario
busca aparece empujado hacia abajo.

Las guías de diseño de CLI son explícitas en las dos cosas que faltan: **empieza por
ejemplos** y **muestra primero lo más común**. Y la clasificación de comandos que hace falta
para agrupar **ya existe**: se hizo al inventariar el contrato de salida.

Backlog: **UX-04**.

## What Changes

- Los comandos se agrupan por propósito —entorno, Magento, base de datos, inspección,
  herramientas, IA— en lugar de listarse alfabéticamente. Los grupos y su orden se declaran
  en `data/command_descriptions.json`, no en el código: añadir un comando será ponerle su
  grupo.
- Una línea de uso al principio y una sección de **ejemplos** de las tareas más frecuentes.
- Un pie con las opciones globales y el puntero a `hm <comando> --help`.
- **Logo nuevo**, de tres líneas en lugar de ocho, con el nombre y el subtítulo a la derecha
  de la marca. Se dibuja con bloques Unicode, con una versión ASCII de respaldo para
  terminales sin UTF-8, y **sólo cuando la salida es un terminal**: hoy también se imprime
  al canalizar, donde es ruido.

## Non-goals

- **No se reparten los comandos en columnas** según la anchura del terminal: eso necesita la
  biblioteca de componentes (UX-07) y llegará después. Una lista por grupo se lee bien a
  cualquier anchura.
- No se toca la ayuda de cada comando (`hm <comando> --help`), que ya funciona.
- No se renombra ningún comando ni se cambia ninguna descripción.
- No se añaden dependencias: nada de `figlet` ni de generadores en tiempo de ejecución, el
  logo es texto estático.

## Capabilities

### New Capabilities
- `command-discovery`: cómo descubre alguien qué sabe hacer Dockergento y por dónde empezar.

### Modified Capabilities
<!-- Ninguna: `cli-performance` sigue vigente y este cambio debe respetar su presupuesto. -->

## Impact

- **Código**: `console/helpers/print_help.sh` (la presentación),
  `console/tasks/copyright.sh` (el logo).
- **Datos**: `data/command_descriptions.json` gana una clave `group` por comando y un bloque
  con el orden de los grupos y los ejemplos.
- **Documentación**: `README.md`.
- **Rendimiento**: `hm --help` está hoy en ~390 ms con 3 procesos `jq`, y hay un presupuesto
  de 800 ms vigilado por una prueba. La agrupación **no puede** volver a introducir una
  consulta por comando.
- **Proyectos existentes**: sin migración. Los comandos personalizados de un proyecto que no
  declaren grupo aparecen en su propia sección.
