## Context

`print_commands_info` recorre los ficheros de `console/commands/`, pide a `jq` la tabla
entera de una vez y la imprime. Esa forma —una consulta, un bucle en Bash— es la que dejó
`hm --help` en ~390 ms desde los 5,7 s que costaba, y hay una prueba de presupuesto que
falla si alguien la deshace.

La clasificación por propósito ya existe: se hizo al inventariar los comandos para el
contrato de salida (informativos, de flujo de datos, de paso, de control). Sirve para
razonar sobre la salida, pero **no es la que quiere ver un usuario**: a quien busca ayuda no
le importa si un comando envuelve su salida en JSON, le importa si sirve para arrancar el
entorno o para tocar la base de datos.

## Goals / Non-Goals

**Goals:**
- Que alguien que abre la ayuda por primera vez sepa por dónde empezar.
- Que añadir un comando no obligue a tocar el código de la presentación.
- No perder el rendimiento conseguido.

**Non-Goals:**
- Layout en columnas (necesita UX-07).
- Reescribir descripciones.

## Decisions

### 1. Grupos por propósito de usuario, no por naturaleza técnica

Seis grupos, en este orden:

| Grupo | Qué contiene |
|---|---|
| **Environment** | `start`, `stop`, `restart`, `rebuild`, `down`, `setup`, `doctor`, `describe`, `list` |
| **Magento** | `magento`, `composer`, `install`, `purge`, `npm`, `grunt`, `test-unit`, `test-integration`, `n98-magerun` |
| **Database** | `mysql`, `mysqldump`, `transfer-db`, `masquerade` |
| **Files** | `copy-to-container`, `copy-from-container`, `transfer-media` |
| **Tools** | `bash`, `exec`, `ssl`, `set-host`, `debug-on`, `debug-off`, `varnish-on`, `varnish-off`, `config-env`, `compatibility`, `docker-compose`, `docker-stop-all`, `update`, `cloud`, `cloud-login`, `create-project` |
| **AI** | `ai-init`, `ai-pull`, `ai-reset` |

Va en `data/command_descriptions.json`, con una clave `group` por comando y un bloque
`_groups` que fija el orden y el título de cada uno. El código no conoce ningún grupo: si
mañana se quiere separar "Testing" de "Magento", es un cambio de datos.

Un comando sin `group` cae en un grupo final "Other", que es lo que hace que los comandos
personalizados de un proyecto sigan apareciendo sin tener que declarar nada.

### 2. Ejemplos declarados, no cableados

Un bloque `_examples` en el mismo fichero, con orden y comentario. Lo mismo que los grupos:
el criterio de qué es "lo común" es del equipo y va a cambiar; el código no debe opinar.

Propuesta de partida: `hm start`, `hm magento cache:clean`, `hm mysql -i dump.sql`,
`hm describe`, `hm doctor`.

### 3. Una sola consulta, otra vez

La tentación al agrupar es recorrer los grupos y preguntar por cada uno. Sería volver a los
143 procesos. La consulta única devuelve `grupo<US>nombre<US>descripción<US>mac` ordenado por
el orden declarado de grupos, y el bucle en Bash sólo detecta cuándo cambia el grupo para
imprimir su cabecera.

La prueba de presupuesto y la de conteo de procesos que ya existen cubren esto.

### 4. El logo: tres líneas, bloques con respaldo ASCII, y sólo con terminal

El actual ocupa ocho líneas dibujadas con `#`, y se imprime siempre, también cuando la
salida va a un fichero o a otro programa.

El nuevo son tres líneas y lleva el nombre y el subtítulo a la derecha de la marca, así que
identifica la herramienta ocupando menos de la mitad:

```
█  █ █▀▄▀█   Hiberus Dockergento
█▀▀█ █ ▀ █   Docker environments for Magento 2
▀  ▀ ▀   ▀
```

Dos condiciones:

- **Respaldo ASCII.** Los bloques (`█`, `▀`) necesitan una configuración regional UTF-8 y una
  fuente que los tenga. Si `LC_ALL`, `LC_CTYPE` o `LANG` no indican UTF-8, se dibuja una
  versión equivalente con caracteres ASCII. Un logo que sale como interrogantes es peor que
  no tener logo.
- **Sólo con terminal.** Es decoración: si stdout no es un TTY, no se imprime. Esto ya lo
  decide la misma lógica que el color, así que no hay una segunda regla que mantener.

Alternativa descartada: generar el logo con `figlet` en tiempo de ejecución. Sería una
dependencia para dibujar texto que no cambia nunca.

### 5. Orden dentro de la pantalla

Logo → uso → grupos → ejemplos → opciones globales → puntero a la ayuda por comando.

El logo va primero, que es como se mostró al elegirlo. Cabe porque son tres líneas: el
argumento de "que no empuje el contenido" se resolvió recortando el logo, no moviéndolo.

### 6. mac y linux

Sin diferencias. La detección de UTF-8 mira las variables de entorno, que se comportan igual
en ambos.

## Risks / Trade-offs

- **Volver a introducir una consulta por comando** → decisión 3, y hay dos pruebas que
  fallan si ocurre.
- **Un comando nuevo sin grupo** → cae en "Other" y se ve; no desaparece.
- **Bloques Unicode ilegibles** → respaldo ASCII por configuración regional.
- **Discutir los grupos eternamente** → están en datos: cambiarlos es editar un JSON, no
  reabrir el diseño.

## Migration Plan

Ninguna. Cambia la presentación, no el comportamiento.

## Open Questions

Ninguna. El logo lo eligió el equipo entre cuatro propuestas.
