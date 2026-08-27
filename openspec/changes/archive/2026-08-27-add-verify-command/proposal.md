# Un comando que dice si lo que se ha escrito está bien

## Por qué

Con agentes escribiendo código, **el cuello de botella deja de ser generar y pasa a ser
verificar**. Un agente puede producir en un minuto lo que lleva media hora revisar, y si la
revisión depende de que alguien se acuerde de ejecutar cuatro herramientas distintas con cuatro
sintaxis distintas, no se hace.

Y no sólo con agentes: hoy, comprobar un cambio en un proyecto de Magento significa recordar si
este proyecto tiene PHPStan, con qué configuración, y si el estándar de PHPCS es el de Magento o
el que alguien dejó a medias.

## Lo que hace falta saber antes de diseñarlo

Las herramientas **no son las mismas en cada proyecto**. En esta máquina, de catorce proyectos:

| Herramienta | Cuántos la tienen |
|---|---|
| PHPUnit | 10 |
| PHP-CS-Fixer | 9 |
| PHPStan | 6 |
| Estándar de código de Magento | 5 |
| Ninguna | 3 |

Un comando con una lista fija de comprobaciones fallaría en la mayoría de los proyectos. Tiene que
**mirar qué hay y ejecutar lo que haya**.

## Qué cambia

- **`hm verify`** ejecuta las comprobaciones que el proyecto tenga instaladas y resume el resultado.
  Lo que no está, se informa como omitido, no como fallo.
- **`--changed`** limita la revisión a los ficheros tocados respecto a la rama base, que es lo que
  interesa al cerrar una tarea.
- **`--all`** añade las lentas: pruebas unitarias y compilación de DI.
- **`--json`** para que un agente o un trabajo de CI lo consuma sin interpretar texto.
- El código de salida distingue «todo bien» de «hay fallos», para poder encadenarlo.

## Qué no cambia

- No se instala ninguna herramienta ni se modifica el `composer.json` de nadie.
- No se corrige nada automáticamente: informa, no toca.
- `hm test-unit` y `hm test-integration` siguen como están.

## Cómo se sabrá que funciona

- En un proyecto sin herramientas, informa de que no hay nada que ejecutar y no falla.
- En uno con PHPStan y PHPCS, ejecuta ambas y falla si alguna encuentra problemas.
- Un error de sintaxis introducido a propósito lo detecta siempre, tenga el proyecto lo que tenga.
- `--changed` no revisa ficheros que no se han tocado.
