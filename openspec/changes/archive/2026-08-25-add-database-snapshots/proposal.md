# Copias de la base de datos con nombre

## Por qué

Antes de un `setup:upgrade`, de importar un dump de producción o de probar una migración, lo
sensato es guardar la base de datos. Hoy eso significa acordarse de:

```bash
hm mysqldump ~/copias/antes-del-upgrade-2026-08-25.sql
```

…elegir dónde ponerlo, inventarse el nombre, y recordar cuál era cuál dos días después. Y para
volver, buscar el fichero y pasarlo por `hm mysql -i`. Funciona, pero es tanto trabajo manual que
en la práctica **no se hace**, y por eso perder una base de datos local sigue siendo posible con un
`down -v` a destiempo.

Falta el equivalente de `ddev snapshot`: guardar con un nombre, ver lo que hay y volver a uno.

## Qué cambia

- **`hm db snapshot [--name=<nombre>]`**: guarda la base de datos con nombre y fecha, sin parar
  nada. Sin nombre, se usa la fecha.
- **`hm db list`**: qué copias hay de este proyecto, cuándo se hicieron y cuánto ocupan.
- **`hm db restore <nombre>`**: vuelve a una. Es destructivo, así que pide confirmación salvo que
  se le diga que no pregunte.
- **`hm db remove <nombre>`**: borra una copia.
- Las copias viven **fuera del proyecto**, en `~/.hm/snapshots/<proyecto>/`: ningún `.gitignore` que
  tocar, nada que se cuele en un commit, y **sobreviven a `hm down -v`** — que es justo cuando se
  necesitan.

## Qué no cambia

- `hm mysqldump` y `hm mysql -i` siguen igual, para exportar a una ruta concreta o importar de
  fuera. Esto es otra cosa: copias con nombre, del proyecto, gestionadas por la herramienta.
- No se toca el ciclo de vida de los contenedores. Que `hm stop` ofrezca guardar una copia, y que
  `down -v` avise, es el cambio siguiente.

## Una desviación de lo que proponía el backlog, y por qué

El backlog proponía `mariadb-backup`, copia física en caliente. Se descarta por el propio criterio
de aceptación que venía escrito al lado: *«crear y restaurar sin parar el proyecto»*.

Una copia física se **crea** en caliente, sí, pero **restaurarla exige parar el servidor** y
sustituir su directorio de datos: es copiar ficheros por debajo de un motor que no puede estar
mirando. Además queda atada a la versión de MariaDB con la que se hizo.

Un volcado lógico —el que ya usa `hm mysqldump`— se crea en caliente con `--single-transaction` y
se restaura con el proyecto en marcha. Es más lento y ocupa más antes de comprimir, y a cambio
cumple lo que se pedía y sirve entre versiones distintas de MariaDB.

## Cómo se sabrá que funciona

- Crear una copia no interrumpe el proyecto ni cambia la base de datos.
- Restaurar deja la base de datos exactamente como estaba al copiar, incluidas las tablas que se
  hayan creado después y que deben desaparecer.
- Las copias siguen ahí después de un `hm down -v`.
- Una copia de otro proyecto no aparece en la lista de este.
- Restaurar sin confirmar no borra nada.
