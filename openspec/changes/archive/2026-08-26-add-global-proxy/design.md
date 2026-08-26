## Context

Cada proyecto publica siete puertos en la máquina, así que dos proyectos no caben a la vez. Es el
bloqueo del que cuelga toda la mitad de abajo del roadmap: un entorno por worktree, varios agentes,
el panel web.

Antes de decidir nada se comprobó, ejecutándolo, qué puede y qué no puede hacer un proxy:

| Comprobación | Resultado |
|---|---|
| ¿Traefik enruta TCP por nombre? | **No sin TLS**: `invalid rule: "HostSNI(\`a.local\`)", has HostSNI matcher, but no TLS on router` |
| ¿Se pueden añadir puertos de escucha con etiquetas? | **No**: `EntryPoint doesn't exist`. Son configuración estática, y un puerto nuevo obliga a reiniciar el proxy de todos |
| ¿macOS tiene más loopbacks que `127.0.0.1`? | **No**: `Can't assign requested address`. Cada alias exige `sudo` y no sobrevive a un reinicio |
| ¿Un túnel bajo demanda alcanza una base de datos no publicada? | **Sí**: MariaDB saluda desde el puerto temporal |
| ¿Compose sabe *quitar* puertos desde un fichero superpuesto? | **Sí**, con `!reset` (Compose 2.24+) |

Los dos primeros descartan enrutar MySQL y AMQP. El último decide toda la forma de la
implementación.

## Goals / Non-Goals

**Goals**
- Dos o más proyectos levantados a la vez, cada uno en su dominio.
- Que quien no lo active no note absolutamente nada.
- Que activarlo sea una propiedad, no una migración.

**Non-Goals**
- No se enruta MySQL ni AMQP por nombre: no se puede sin TLS en la base de datos, y eso es
  demasiada maquinaria para un entorno local.
- No se toca ningún proyecto existente.
- No se sustituye `hm set-host`: el dominio sigue apuntando a `127.0.0.1` en `/etc/hosts`. Quitar
  ese paso es NET-03.

## Decisions

### 1. El proxy es un proyecto de Compose más, en `~/.hm/proxy/`

Con su `docker-compose.yml` generado por la herramienta y nombre de proyecto `hm-proxy`. Sale en
`docker ps` con un nombre reconocible, se puede parar a mano sin magia, y vive donde ya viven la
caché y las copias: fuera de cualquier proyecto.

Los proyectos se conectan a él por una red de Docker compartida, `hm-gateway`, que crea el propio
proxy al levantarse.

### 2. La plantilla no se duplica: se superpone

Compose 2.24 acepta `!reset`, que **quita** una clave en lugar de fusionarla. Comprobado con 2.34:
`ports: !reset []` deja el servicio sin puertos publicados y `hitch: !reset null` elimina el
servicio entero.

Así que la plantilla base **no cambia**. Cuando el proyecto usa el proxy se genera además un
`docker-compose.proxy.yml` que:

- quita los puertos publicados de todos los servicios,
- pone las etiquetas de Traefik en los que se enrutan,
- añade la red compartida,
- elimina `hitch`.

Y el router añade ese fichero al `-f` cuando corresponde. Una sola plantilla que mantener, y el
proxy es puramente aditivo.

El precio: **el proxy exige Compose 2.24 o superior**. Se comprueba antes de generar y se dice, en
lugar de fallar con un error de YAML incomprensible.

### 3. Hitch desaparece cuando hay proxy

`hitch` está en el stack por un motivo concreto: darle HTTPS a Varnish, que no lo tiene. Si Traefik
termina TLS, `hitch` no resuelve ya ningún problema. La cadena pasa de
`hitch → varnish → nginx` a `traefik → varnish → nginx`.

Es un contenedor menos que en producción, y es correcto: en producción quien termina TLS es el
balanceador, no un contenedor del proyecto. El proxy se parece más a producción que lo que teníamos.

### 4. Subdominios para lo auxiliar, y por eso el certificado es comodín

`mail.proyecto.local`, `queue.proyecto.local`, `search.proyecto.local`. Cada interfaz cree que está
en su raíz, que es como esperan estarlo: servidas bajo un prefijo, muchas generan enlaces absolutos
y se rompen.

Eso obliga a un certificado para `*.proyecto.local`, no uno por dominio — que es NET-02, y por eso
va aquí y no después: unos subdominios en HTTPS sin comodín no funcionan, y un Magento con
`base_url` en HTTP no es un entorno realista.

`mkcert` ya se usa y admite comodines. Los certificados viven junto al proxy y Traefik los carga
con su proveedor de ficheros.

### 5. Lo que no es HTTP se abre bajo demanda

`hm tunnel <servicio>` levanta un contenedor efímero conectado a la red del proyecto que publica un
puerto libre en `127.0.0.1` y reenvía al servicio. Comprobado contra una base de datos sin puertos
publicados.

Genérico a propósito: `hm tunnel db` es el caso de todos los días y `hm tunnel rabbitmq` está ahí
sin haber tenido que preverlo. Un solo mecanismo, sin casos especiales.

El puerto lo elige el sistema, así que pueden convivir varios túneles a la vez, y el comando dice
cuál ha tocado.

### 6. El proxy se levanta solo

`hm start` en un proyecto con proxy lo levanta si no lo está. Nadie tiene que acordarse de nada, y
`hm proxy up|down|status` existe para mirarlo y pararlo a mano.

No se para solo: pararlo al detener un proyecto tumbaría los demás.

### 7. Activarlo es una propiedad

`USE_PROXY` en las propiedades del proyecto, `false` por defecto. Cambiarla y regenerar es todo.
Ninguna migración, ningún asistente, y el camino de vuelta es el mismo al revés.

## Risks / Trade-offs

- **Compose 2.24+ obligatorio** para los proyectos con proxy. Se comprueba y se dice.
- **Un punto único de fallo**: si el proxy se cae, se caen todos los sitios. A cambio, `hm doctor`
  puede verlo, cosa que con siete puertos por proyecto no podía.
- **Dos caminos que mantener** —con y sin proxy— mientras dure la convivencia. Es el precio de que
  nadie tenga que migrar, y se paga a gusto.
- **El túnel es un paso más** antes de abrir TablePlus. La alternativa era `sudo` por proyecto o
  puertos que recordar.

## Migration Plan

Ninguna obligatoria. Para activarlo en un proyecto:

```bash
# en config/docker/properties.json: "USE_PROXY": "true"
hm setup -f
hm rebuild
```

Y para volver, lo mismo con `false`.

## Open Questions

Ninguna bloqueante. Queda por decidir más adelante si el proxy pasa a ser el comportamiento por
defecto, lo que sería una 2.0.0 y no toca ahora.
