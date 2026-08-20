## 1. Medición

- [x] 1.1 Añadir a `tests/` un ayudante de medición que devuelva el mejor de tres intentos para una orden dada
- [x] 1.2 Registrar en el propio cambio la línea base medida por ruta y por plataforma
- [x] 1.3 Añadir un modo de conteo de procesos (envoltorios de `jq`, `docker` y `git` en el `PATH`) para poder verificar las mejoras por número de procesos y no sólo por tiempo

## 2. Listado de comandos en una sola pasada

- [x] 2.1 Reescribir `print_commands_info` para obtener nombre, descripción y marca de plataforma de todos los comandos con una única invocación de `jq`
- [x] 2.2 Mantener idéntica la salida actual, incluidos colores, alineación y el filtrado de comandos exclusivos de macOS
- [x] 2.3 Aplicar lo mismo al listado de comandos personalizados del proyecto
- [x] 2.4 Comprobar que `hm --help` baja de 500 ms y lanza un solo `jq`

## 3. Coste fijo perezoso

- [x] 3.1 Convertir la detección de Docker Compose (`get_docker_compose_cmd` y `get_docker_compose_version`) en cálculo perezoso memorizado
- [x] 3.2 Convertir la versión de `hm` y la versión de Magento en cálculo perezoso memorizado
- [x] 3.3 Forzar el cálculo en los comandos que crean o recrean contenedores, antes de invocar a Compose, para que las etiquetas `hm.*` sigan completas
- [x] 3.4 Verificar que la memorización se asigna en el shell del llamante y no dentro de una sustitución de comando
- [x] 3.5 Comprobar con `docker inspect` que un contenedor recién creado conserva `hm.magento` y `hm.version`
- [x] 3.6 Fusionar las tres llamadas a `git rev-parse` de la resolución de worktree en una sola, usando su fallo como señal de "no es un repositorio"
- [x] 3.7 Comprobar que las 9 pruebas unitarias y las 26 aserciones de integración de worktree siguen pasando sin cambios
- [x] 3.8 Comprobar que el suelo de arranque baja de 300 ms a ~215 ms

## 4. Caché de validación de Compose, fuera del repositorio

- [x] 4.1 Crear `~/.hm/cache/` y derivar el nombre del fichero de la ruta del proyecto
- [x] 4.2 Escribir la marca de tiempo de la última validación correcta y comparar contra los ficheros de Compose
- [x] 4.3 Revalidar en cuanto cualquiera de esos ficheros cambie de fecha
- [x] 4.4 Tratar una caché ausente, ilegible o corrupta como si no existiera
- [x] 4.5 Hacer que `hm doctor` valide siempre, ignorando la caché
- [x] 4.6 Comprobar que una configuración que se rompe tras una validación correcta se detecta en el comando siguiente
- [x] 4.7 Comprobar que ejecutar comandos no deja ningún fichero nuevo en el repositorio del proyecto
- [x] 4.8 Añadir la limpieza de `~/.hm/cache/` a lo que barre `hm clean` (CLI-08)

## 5. Diagnóstico concurrente

- [x] 5.1 Lanzar las comprobaciones en paralelo, cada una escribiendo en su propio fichero temporal
- [x] 5.2 Recoger los resultados en orden de nombre para que el informe sea determinista
- [x] 5.3 Conservar el límite de tiempo y el aislamiento de fallos por comprobación
- [x] 5.4 Comprobar que el informe es idéntico al de la ejecución secuencial
- [x] 5.5 Comprobar que `hm doctor` baja de 2 segundos

## 6. Presupuesto vigilado

- [x] 6.1 Añadir una suite de rendimiento con los presupuestos de `hm --help`, `hm --version` y `hm doctor`
- [x] 6.2 Hacer que la suite se pueda saltar explícitamente en máquinas muy lentas, informando de por qué
- [x] 6.3 Documentar los presupuestos y cómo medirlos

## 7. Documentación

- [x] 7.1 Documentar en `docs/` el criterio de rendimiento: el coste de la CLI es el número de procesos que lanza
- [x] 7.2 Añadir la entrada al changelog con las cifras de antes y después
- [x] 7.3 Marcar PERF-01 a PERF-04 en `docs/research/backlog.md` con enlace a este cambio
