# Que no se pueda perder una base de datos por escribir un comando de más

## Por qué

Tres comandos de esta herramienta destruyen trabajo sin preguntar nada:

**`hm down -v`** borra los volúmenes del proyecto, y con ellos la base de datos. Es una letra de
diferencia con `hm down`, que no borra nada. No hay confirmación, no hay aviso, no hay vuelta.
Durante el desarrollo de esta misma versión se destruyó así un entorno real de esta máquina:
siete volúmenes, incluida la base de datos, con un solo comando y sin una sola pregunta.

**`hm docker-stop-all`** para **todos los contenedores de la máquina**, no los del proyecto. Los de
otros proyectos, y también los que no tienen nada que ver con Dockergento. Su nombre lo dice, pero
se escribe a menudo pensando en «parar esto».

**`hm stop`** no destruye nada, pero es el momento en el que más barato sale guardar una copia: se
está dejando el proyecto, y a menudo es justo antes de un `down -v`.

Ahora que `hm db snapshot` existe, hay dónde guardar antes de romper.

## Qué cambia

- **`hm down -v` pregunta antes**, dice qué volúmenes va a borrar y ofrece guardar una copia de la
  base de datos primero. `hm down` sin `-v` sigue sin preguntar, porque no destruye nada.
- **`hm stop --snapshot`** guarda una copia antes de parar.
- **`hm docker-stop-all` pregunta**, y dice cuántos contenedores va a parar y cuántos son de otros
  proyectos.
- Las tres confirmaciones se saltan con la opción global `--yes`, para guiones y automatismos.

## Qué no cambia

- `hm down` sin `-v`, `hm stop` sin opciones y todo lo demás siguen exactamente igual.
- No se hacen copias automáticas a espaldas de nadie: guardar es siempre una elección.
- El comportamiento sin terminal —CI, agentes— es el de siempre, porque ahí no se pregunta.

## Cómo se sabrá que funciona

- `hm down -v` sin confirmar deja los volúmenes intactos.
- Aceptar la copia deja un snapshot restaurable antes de borrar nada.
- `hm docker-stop-all` sin confirmar no para nada, y cuenta bien lo que iba a parar.
- Con `--yes`, los tres se comportan como hoy.
