# Un panel que se siente instantáneo

## Por qué

`hm tui` funciona y es correcto, pero se usa mal: parpadea en cada tecla y responde tarde.
Medido en esta máquina, con diez entornos:

| Coste | Medida |
|---|---|
| Un fotograma completo | **404 ms** |
| └ `tui_fleet_rows` (1 `jq` + ~30 subshells de recorte) | 179 ms |
| └ `tui_fleet_header` + `tui_fleet_count` (dos `jq` por fotograma) | ~198 ms |
| └ `tui_doctor_lines` | 27 ms |

Y el parpadeo tiene una causa concreta y distinta de la lentitud: `draw` borra la pantalla
entera con `\e[2J` antes de redibujar, así que el terminal muestra una pantalla **vacía**
durante un refresco. A eso se le suman decenas de `printf` por fotograma, que el terminal va
pintando a medida que llegan.

El problema no es el lenguaje: es que el panel recalcula el layout desde JSON cada vez que se
pulsa una tecla, cuando los datos no han cambiado. Mover la selección una fila debería
repintar dos líneas sin lanzar un solo proceso.

## Qué cambia

- El layout se calcula **cuando llegan los datos**, no cuando se pulsa una tecla. Un
  fotograma pasa a ser ensamblado de cadenas ya calculadas.
- Nunca más borrar la pantalla entera entre fotogramas: cursor al origen, sobrescribir línea
  a línea borrando el resto de cada una, y limpiar lo que sobra por debajo sólo al final.
- Un fotograma es **una** escritura, no cuarenta.
- Los fotogramas se emiten dentro de *synchronized output* (`\e[?2026h`/`\e[?2026l`), de modo
  que un terminal moderno no muestra nunca medio fotograma.
- Mover la selección repinta sólo las líneas afectadas.
- Las teclas pendientes se descartan antes de redibujar, para que mantener `j` pulsado no
  encole treinta fotogramas.

## Qué no cambia

- Ninguna dependencia nueva: sigue siendo Bash 3.2 y secuencias ANSI. Ni `fzf`, ni `gum`, ni
  `dialog`.
- Las teclas, las vistas y las acciones son las mismas: esto no añade funcionalidad.
- El contrato JSON de `list`, `describe` y `doctor` no se toca.
- Las funciones de presentación siguen siendo puras y comprobables sin terminal.

## Cómo se sabrá que funciona

- Un fotograma cuesta menos de 20 ms con veinte entornos, verificado por un test de
  presupuesto como el de `hm --help`.
- Mover la selección no vuelve a ejecutar `jq` ni a lanzar subshells: comprobable contando
  procesos.
- El terminal nunca recibe `\e[2J` entre fotogramas.
- Con veinte teclas encoladas de golpe, el número de fotogramas emitidos es mucho menor que
  veinte.
