## Context

Los tres comandos que destruyen trabajo lo hacen porque son envoltorios de una línea sobre Docker
Compose: `$DOCKER_COMPOSE down "$@"` pasa lo que le den y se aparta. Eso es una virtud —no hay
sorpresas— hasta que el argumento es `-v`.

El caso no es hipotético. Durante esta misma versión, probando los guardarraíles de worktree, un
`hm down -v` destruyó los siete volúmenes de un entorno real de esta máquina, con su base de datos.
No hubo aviso porque no hay ninguno.

Ahora existe `hm db snapshot`, así que por primera vez se puede ofrecer algo mejor que «¿seguro?»:
se puede ofrecer guardar.

## Goals / Non-Goals

**Goals**
- Que destruir requiera una respuesta deliberada.
- Que la salida barata —guardar una copia— esté al alcance en el momento en que hace falta.
- Que nada de esto estorbe en un guion.

**Non-Goals**
- No se hacen copias automáticas. Una copia que nadie pidió ocupa disco, tarda, y enseña a ignorar
  mensajes.
- No se toca lo que no destruye: `hm down` a secas, `hm stop` a secas.
- No se protege `hm mysql -q "DROP ..."` ni nada dentro de la base de datos: esto es sobre el ciclo
  de vida de los contenedores.

## Decisions

### 1. Sólo pregunta lo que destruye

`down -v` sí; `down` no. `docker-stop-all` sí; `stop` no. La regla es «pregunta si al terminar
falta algo que no se puede recuperar», y aplicarla con más amplitud —confirmar cualquier `down`—
convierte la confirmación en un trámite que se responde sin leer, que es exactamente cómo dejan de
funcionar las confirmaciones.

Parar contenedores no destruye nada, pero `docker-stop-all` alcanza **toda la máquina**: puede
tumbar el entorno de otra persona en mitad de algo. Ahí lo que se protege no es un dato, es el
trabajo de otro.

### 2. La pregunta enseña lo que se va a perder

Una confirmación que dice «¿estás seguro?» no aporta información: la respuesta ya estaba decidida
al escribir el comando. La que dice **qué volúmenes** y **cuántos contenedores de otros proyectos**
sí, porque puede contradecir lo que se creía.

`down -v` enumera los volúmenes por nombre. `docker-stop-all` cuenta los del proyecto actual y los
demás por separado.

### 3. `down -v` ofrece guardar, no guarda

Tres respuestas, no dos: guardar y borrar, borrar sin guardar, o no hacer nada. La primera es la
opción por defecto y la que se acepta con Enter, porque es la que nadie lamenta.

Guardar automáticamente sería peor: en un proyecto que se destruye a propósito —un entorno de
pruebas que se recrea diez veces al día— dejaría diez copias que nadie va a mirar.

### 4. `stop --snapshot`, no `stop` que siempre guarda

Parar es una operación cotidiana y rápida; que a veces tarde un minuto porque está volcando una
base de datos sería una sorpresa desagradable. La copia se pide.

Se llama igual que el subcomando que la crea (`db snapshot`), para que sea una sola cosa con un
solo nombre.

### 5. Sin terminal, el comportamiento de siempre

En un guion, en CI o con un agente no hay a quién preguntar, y una herramienta que se queda
esperando una respuesta que nunca llega es peor que una que destruye: al menos la segunda termina.
Ahí se aplica lo que ya decide la CLI: sin terminal o con `--yes`, no se pregunta.

Es una decisión con filo, y es la correcta: quien automatiza `down -v` lo ha escrito a propósito.

### 6. Las confirmaciones no piden escribir nada

A diferencia de `db restore` y `db clear`, aquí basta con responder. La diferencia es qué se pierde:
una copia borrada es irrecuperable y es la única, mientras que un entorno destruido se vuelve a
levantar —lento y molesto, pero se recupera— y ahora además puede haberse guardado antes.

Pedir escribir el nombre del proyecto en cada `down -v` acabaría en que la gente lo escribe sin
leer, y habría gastado la única señal fuerte que tenemos en el caso menos grave.

## Risks / Trade-offs

- **Una pregunta más en un comando frecuente.** `down -v` no es frecuente; `down` a secas lo es y no
  pregunta.
- **La copia tarda** y se hace con el entorno aún en pie, así que el usuario espera. Se avisa antes.
- **`docker-stop-all` es más lento de escribir ahora.** Es la intención: alcanza toda la máquina.
- **Con `--yes` se puede destruir en silencio.** Es lo que se pide al escribirlo.

## Migration Plan

Ninguna. Los guiones que usen estos comandos sin terminal se comportan igual; los interactivos
ganan una pregunta.

## Open Questions

Ninguna.
