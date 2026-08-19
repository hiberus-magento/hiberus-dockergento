## Why

`hm` asume que quien lo ejecuta es una persona delante de un terminal, y eso ya nos ha
mordido en producción: hasta la versión 1.4.5, `hm mysql` trataba cualquier stdin sin tty
como un volcado a importar, de modo que `hm mysql -q "SELECT ..."` lanzado desde CI o
desde un agente de IA nunca llegaba al parser de opciones. Ese bug era el síntoma de un
patrón que recorre toda la CLI: entrada interactiva implícita, salida pensada sólo para
leerse con los ojos y un `exit 1` genérico para cualquier fallo.

Sin resolver esto, ninguna de las piezas siguientes del backlog es fiable: `hm describe`,
`hm list`, `hm doctor`, el TUI, `hm verify` y `hm mcp` dependen todas de que la CLI se
comporte de forma predecible cuando quien la llama es una máquina.

Backlog: **CLI-01**.

## What Changes

- Nuevo componente de salida estructurada en `console/components/` con funciones para
  emitir objetos y errores JSON, construidos con `jq` (ya es dependencia del proyecto).
- Convención `--json` para los comandos de lectura. Cuando stdout **no** es un TTY, el
  formato por defecto pasa a ser JSON; cuando lo es, se mantiene la salida de texto actual.
  `--no-json` fuerza texto en cualquier caso.
- Modo no interactivo: flag global `--yes` y variable `HM_NON_INTERACTIVE=1`. Con él
  activado, ninguna función de `console/components/input_info.sh` se queda esperando
  entrada: se usa el valor por defecto o se falla con un error accionable si no lo hay.
- Códigos de salida documentados y estables, sustituyendo el `exit 1` genérico:
  `0` correcto, `1` error genérico, `2` argumentos inválidos, `3` Docker no disponible,
  `4` proyecto no configurado, `5` servicio no levantado.
- Errores estructurados por stderr cuando el modo es JSON, con tipo de error, mensaje y
  acción sugerida.
- Auditoría y corrección de todos los `[ -t 0 ]` y `read -rp` del repositorio para que
  respeten el modo no interactivo.
- Documentación de la convención en `README.md` y en `docs/`, para que sea el contrato al
  que se acojan los comandos futuros.

**BREAKING** (menor, acotado): los comandos que hoy siempre terminan con `exit 1` pasarán a
devolver códigos específicos. Scripts propios que comprueben `$? -eq 1` de forma estricta
tendrán que ajustarse; comprobar "distinto de 0" sigue funcionando igual.

## Non-goals

- No se implementa ningún comando nuevo. `describe`, `list` y `doctor` van en cambios
  posteriores y consumirán este contrato.
- No se define aún el esquema de datos de `describe`/`list`: aquí sólo se fija la
  *envoltura* (formato, errores, códigos de salida), no el contenido.
- No se toca el comportamiento interactivo por defecto: una persona en un terminal debe
  ver exactamente lo mismo que hoy.
- No se añade ninguna dependencia nueva: se resuelve con Bash y `jq`.

## Capabilities

### New Capabilities
- `cli-output-contract`: cómo emite `hm` sus resultados y sus errores, cómo decide entre
  texto y JSON, cómo se comporta sin terminal interactiva y qué códigos de salida devuelve.

### Modified Capabilities
<!-- Ninguna: openspec/specs/ está vacío; este es el primer contrato formalizado. -->

## Impact

- **Código**: `bin/run` (parseo de flags globales y exportación del modo),
  `console/components/print_message.sh`, `console/components/input_info.sh`,
  nuevo `console/components/print_json.sh`, `console/helpers/process_hm_options.sh`,
  `console/helpers/docker.sh` (códigos de salida) y todos los `console/commands/*.sh` que
  hoy usan `read` o `[ -t 0 ]` (al menos `mysql.sh`, `setup.sh`, `install.sh`,
  `create-project.sh`, `transfer-db.sh` y los `ai-*.sh`).
- **Datos**: `data/command_descriptions.json` para documentar los flags globales.
- **Documentación**: `README.md` y `docs/`.
- **Proyectos existentes**: no requiere migración. No cambia ficheros del proyecto ni la
  configuración de Docker; sólo el comportamiento de la CLI, y el modo interactivo se
  mantiene idéntico.
- **Dependencias**: ninguna nueva (`jq` ya es requisito).
