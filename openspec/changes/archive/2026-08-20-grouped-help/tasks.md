## 1. Datos

- [x] 1.1 Añadir la clave `group` a cada uno de los comandos de `data/command_descriptions.json`
- [x] 1.2 Añadir el bloque `_groups` con el identificador, el título y el orden de cada grupo
- [x] 1.3 Añadir el bloque `_examples` con los ejemplos y su explicación
- [x] 1.4 Comprobar que el JSON sigue siendo válido y que ningún comando se queda sin entrada

## 2. Presentación

- [x] 2.1 Reescribir `print_commands_info` para emitir grupo, nombre, descripción y marca de plataforma en **una sola** consulta, ordenado por el orden declarado
- [x] 2.2 Imprimir la cabecera de grupo al cambiar de grupo, en el bucle de Bash
- [x] 2.3 Enviar al grupo de no clasificados los comandos sin `group`
- [x] 2.4 Mantener el listado de comandos personalizados del proyecto
- [x] 2.5 Añadir la línea de uso al principio
- [x] 2.6 Añadir la sección de ejemplos
- [x] 2.7 Añadir el pie con las opciones globales y el puntero a la ayuda por comando

## 3. Logo

- [x] 3.1 Sustituir el logo de ocho líneas por el de tres, con nombre y subtítulo a la derecha
- [x] 3.2 Añadir la versión ASCII de respaldo
- [x] 3.3 Elegir una u otra según si la configuración regional indica UTF-8
- [x] 3.4 No dibujar el logo cuando la salida no es un terminal
- [x] 3.5 Comprobar que el logo respeta la decisión de color que ya existe

## 4. Verificación

- [x] 4.1 Prueba de que todos los comandos existentes aparecen en la ayuda
- [x] 4.2 Prueba de que un comando sin grupo aparece en el grupo de no clasificados
- [x] 4.3 Prueba de que el orden de los grupos es el declarado
- [x] 4.4 Prueba de que sin UTF-8 no se emiten caracteres de bloque
- [x] 4.5 Prueba de que con la salida canalizada no aparece el logo
- [x] 4.6 Comprobar que el presupuesto de rendimiento y el conteo de procesos siguen pasando
- [x] 4.7 Verificar en mac y en linux

## 5. Documentación

- [x] 5.1 Actualizar `README.md` con el aspecto nuevo de la ayuda
- [x] 5.2 Documentar en `docs/` cómo declarar el grupo de un comando nuevo
- [x] 5.3 Añadir la entrada al changelog
- [x] 5.4 Marcar UX-04 en `docs/research/backlog.md` con enlace a este cambio
