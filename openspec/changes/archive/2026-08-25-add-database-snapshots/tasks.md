## 1. Dónde viven

- [x] 1.1 `~/.hm/snapshots/<proyecto>/`, agrupadas por el nombre resuelto del proyecto
- [x] 1.2 Crear el directorio al vuelo, sin escribir nada dentro del proyecto
- [x] 1.3 Validar el nombre: sólo lo que puede formar un nombre de fichero

## 2. `hm db snapshot`

- [x] 2.1 Volcado en caliente con `--single-transaction`, comprimido
- [x] 2.2 Incluir rutinas, disparadores y eventos, no sólo las tablas
- [x] 2.3 Nombre por defecto con fecha y hora
- [x] 2.4 Negarse a sobrescribir salvo que se indique
- [x] 2.5 Anotar dentro del volcado la versión de Magento y la fecha

## 3. `hm db list`

- [x] 3.1 Nombre, fecha y tamaño de cada copia del proyecto
- [x] 3.2 Sólo las del proyecto actual
- [x] 3.3 Mensaje útil cuando no hay ninguna
- [x] 3.4 Salida JSON con los mismos datos

## 4. `hm db restore`

- [x] 4.1 Vaciar el esquema antes de cargar, para que la restauración sea exacta
- [x] 4.2 Confirmación escribiendo el nombre del proyecto
- [x] 4.3 Respetar la opción global de no preguntar
- [x] 4.4 Fallar nombrando las copias existentes cuando la pedida no está

## 5. `hm db remove`

- [x] 5.1 Borrar la copia indicada
- [x] 5.2 Fallar sin borrar nada cuando no existe

## 6. Integración

- [x] 6.1 Entrada en `data/command_descriptions.json`, grupo de base de datos
- [x] 6.2 Ayuda del comando con sus subcomandos
- [x] 6.3 Que un subcomando desconocido sea error de uso y liste los válidos

## 7. Verificación

- [x] 7.1 Test de ida y vuelta: copiar, cambiar la base de datos, restaurar y comprobar el estado
- [x] 7.2 Test de que una tabla creada después de la copia desaparece al restaurar
- [x] 7.3 Test de que el proyecto sigue en marcha durante la copia
- [x] 7.4 Test de que las copias de otro proyecto no aparecen
- [x] 7.5 Test de que no se escribe nada dentro del proyecto
- [x] 7.6 Test de que las copias sobreviven a `down -v`
- [x] 7.7 Test de que no confirmar deja la base de datos intacta
- [x] 7.8 Test de nombres inválidos y de copia inexistente
- [x] 7.9 Suite completa en mac y linux

## 8. Documentación

- [x] 8.1 `docs/db.md`
- [x] 8.2 Entrada en el changelog de la 1.6.0
- [x] 8.3 Marcar DB-01 en el backlog
