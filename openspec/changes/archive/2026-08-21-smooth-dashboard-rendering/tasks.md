## 1. Medir antes

- [x] 1.1 Registrar el coste actual del fotograma como línea base en el test de presupuesto
- [x] 1.2 Declarar el presupuesto objetivo del fotograma y de la composición

## 2. Separar componer de pintar

- [x] 2.1 Componer las líneas de la flota una vez al cargar los datos, no al pulsar una tecla
- [x] 2.2 Componer las líneas del detalle al abrirlo
- [x] 2.3 Componer los avisos del doctor con los datos, no en cada fotograma
- [x] 2.4 Guardar el fotograma como array de líneas indexado, no como cadena única
- [x] 2.5 Recomponer al recibir `SIGWINCH`, y sólo entonces
- [x] 2.6 Eliminar del camino de pintado toda llamada a `jq`, subshell y `while read`

## 3. Pintar sin parpadeo

- [x] 3.1 Sustituir el borrado de pantalla por cursor al origen y borrado por línea
- [x] 3.2 Limpiar de la última línea hacia abajo al cerrar el fotograma
- [x] 3.3 Emitir el fotograma completo en una sola escritura
- [x] 3.4 Envolver el fotograma en las marcas de salida sincronizada
- [x] 3.5 Mantener `\e[2J` sólo al entrar en la pantalla alternativa

## 4. Lo que la medida descartó

- [x] 4.1 ~~Repintar sólo las dos filas afectadas~~ — descartado: el fotograma completo mide
  1,7 ms y el parcial ahorraría ~1 ms de un presupuesto de 16 ms por refresco, a cambio de un
  segundo camino de pintado que depende de suposiciones sobre la pantalla (design §6)
- [x] 4.2 ~~Recurrir al fotograma completo en el resto de casos~~ — es el único camino
- [x] 4.3 ~~Descartar las teclas encoladas~~ — imposible en bash 3.2: `read -t 0` es de bash 4 y
  en macOS devuelve 1 aunque haya entrada esperando, comprobado (design §7)
- [x] 4.4 Repintar al redimensionar sin esperar tecla, sacando la lectura de tecla del subshell
- [x] 4.5 Elegir la forma del pie por lo que cabe, no por un umbral de columnas
- [x] 4.6 Desplazar la lista para mantener la selección visible cuando no cabe

## 5. Verificación

- [x] 5.1 Test de que el camino de pintado no emite borrado de pantalla completo
- [x] 5.2 Test de que el fotograma va envuelto en las marcas de salida sincronizada
- [x] 5.3 Test de que pintar no lanza `jq` ni `awk`
- [x] 5.4 Test de presupuesto del fotograma con veinte entornos
- [x] 5.5 Test de que un fotograma más corto no deja restos del anterior
- [x] 5.6 Test de que el fotograma nunca sobrepasa la altura del terminal, ni con la lista
  desplazada ni con la línea que indica qué parte se muestra
- [x] 5.7 Comprobación con pseudo-terminal de que el panel sigue funcionando igual
- [x] 5.8 Comprobación manual sobre los entornos reales, y en mac y linux

## 6. Documentación

- [x] 6.1 Documentar en `docs/tui.md` cómo se dibuja y por qué así
- [x] 6.2 Añadir la entrada al changelog
- [x] 6.3 Crear `docs/research/tui-landscape.md` con la comparativa de TUIs en Bash
- [x] 6.4 Enlazar la comparativa desde el índice de research y desde TUI-01 en el backlog
