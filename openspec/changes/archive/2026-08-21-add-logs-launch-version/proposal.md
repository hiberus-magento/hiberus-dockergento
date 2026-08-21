# Los verbos que faltan: `logs`, `launch`, `version`

## Por qué

Tres huecos de la Ola 2 que el resto del trabajo ha ido dejando a la vista:

**No hay `hm logs`.** Para ver por qué un servicio no arranca hay que escribir
`hm docker-compose logs -f --tail 100 phpfpm`, que exige saber que por debajo hay Compose. Es
la operación más frecuente cuando algo va mal y es la más incómoda de la herramienta.

**No hay `hm launch`.** Abrir la tienda significa recordar el dominio, o mirarlo con
`hm describe`, y escribirlo en el navegador.

**No hay `hm version` como comando.** Existe `hm --version`, que informa de la versión de la
CLI, pero no de la versión de Docker ni de Compose — precisamente los dos datos que hacen falta
en un informe de error. Y quien busca la versión escribe `hm version`, no `hm --version`.

Los dos primeros no son una hipótesis: el panel los necesitó al implementarse, no los encontró,
y se quedó con dos apaños. `o` abría el navegador por su cuenta resolviendo la URL con `jq`, y
`l` llama a `hm docker-compose logs`. Con este cambio el panel deja de rodear a la CLI y llama a
comandos de verdad, que es la regla de la que depende que sus protecciones se apliquen.

## Qué cambia

- **`hm logs [servicio...] [-f] [--tail N] [--since T]`**: los registros del proyecto, o de los
  servicios que se nombren. Su salida es datos, así que nunca va envuelta en JSON. Un nombre de
  servicio que no existe se rechaza con la lista de los que sí, en lugar del error de Compose.
- **`hm launch [--admin] [--mailhog] [--rabbitmq] [--search]`**: abre en el navegador la
  dirección que la CLI ya sabe. Sin terminal, o con `--json`, la escribe en lugar de abrirla:
  así sirve igual en un script.
- **`hm version`**: versión de la CLI con su referencia exacta, más las versiones de Docker y de
  Compose, en texto o en JSON. `hm --version` sigue funcionando igual.
- El panel pasa a usar `hm logs` y `hm launch`, y se queda sin lógica propia de navegador.

## Qué no cambia

- `hm --version` mantiene su salida actual: es lo que consumen los scripts que ya existen.
- Ningún comando nuevo toca contenedores ni datos: los tres son de lectura.
- Nada nuevo que instalar. Abrir el navegador usa `open` en mac y `xdg-open` en Linux, y si no
  hay ninguno se escribe la URL.

## Cómo se sabrá que funciona

- `hm logs` sin argumentos muestra los registros del proyecto, y con un servicio sólo los de ese
  servicio; con un nombre inventado falla con código de servicio y nombra los disponibles.
- `hm logs --json` no envuelve nada: los registros salen tal cual.
- `hm launch --json` devuelve la URL y no abre nada; en un terminal abre el navegador.
- `hm version` informa de CLI, Docker y Compose, en los dos formatos.
- El panel no vuelve a invocar un comando que no existe: el test que lo comprueba pasa a
  encontrar `logs.sh` y `launch.sh`.
