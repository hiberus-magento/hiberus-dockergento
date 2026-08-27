# Decir de una vez qué comandos son seguros

## Por qué

Cuando un agente trabaja con esta herramienta, alguien tiene que decidir qué puede ejecutar sin
preguntar. Hoy esa decisión la toma cada persona por su cuenta, en su propio fichero de
configuración, y ninguna lista se parece a la siguiente.

En la máquina de referencia la configuración dice, literalmente, `Bash` — **todo permitido**. Es
decir: un agente puede ejecutar `hm down -v` y destruir una base de datos sin que nadie lo
autorice. No por descuido, sino porque mantener a mano una lista de sesenta comandos no es
razonable.

## Y hay un problema debajo

La herramienta **ya clasifica sus comandos**, en dos sitios distintos y escritos a mano:

- `hm_alters_environment` — los que el guardarraíl de worktree rechaza.
- `hm_creates_containers` — los que necesitan las etiquetas completas.

Añadir una tercera lista para los permisos sería el momento exacto en que empiezan a
contradecirse. Un comando nuevo hoy exige acordarse de dos sitios; mañana, de tres.

## Qué cambia

- **Cada comando declara su nivel de riesgo** en el mismo fichero donde ya declara su descripción y
  su grupo: el sitio por el que se pasa obligatoriamente al añadir un comando.
- **`hm permissions`** genera la configuración de permisos a partir de esa clasificación, en lugar
  de que cada persona la escriba.
- Las dos clasificaciones que ya existen **se verifican contra esa fuente**, para que no puedan
  divergir en silencio.

## Qué no cambia

- Nadie tiene que usar la configuración generada: se imprime, y quien quiera la aplica.
- Las dos funciones internas siguen siendo listas rápidas en Bash, porque se ejecutan en cada
  invocación y leer un JSON ahí costaría más que todo lo que ahorra.
- No se toca ningún fichero de configuración del usuario.

## Cómo se sabrá que funciona

- Todo comando tiene nivel de riesgo declarado, y hay una prueba que lo exige.
- Lo que el guardarraíl de worktree rechaza coincide con lo clasificado como peligroso o de
  cuidado, comprobado por una prueba.
- La configuración generada permite `hm describe` y exige confirmación para `hm down`.
