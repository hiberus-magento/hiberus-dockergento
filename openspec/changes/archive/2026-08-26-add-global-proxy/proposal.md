# Un proxy para que dos proyectos puedan estar levantados a la vez

## Por qué

Cada proyecto publica hoy los puertos 80, 443, 3306, 9200, 8025, 5672 y 15672 en la máquina. Como
sólo hay un 80 y un 3306, **dos proyectos Dockergento no pueden estar levantados al mismo tiempo**:
hay que parar uno para mirar el otro.

Eso convierte cualquier comparación entre ramas, cualquier revisión de una rama de otra persona y
cualquier trabajo en paralelo en una secuencia de arranques y paradas de varios minutos cada uno.
Y es el bloqueo que impide todo lo que viene detrás: un entorno por worktree, varios agentes
trabajando a la vez, el panel web.

## Qué cambia

- **Un proxy global**, uno por máquina, que enruta por nombre de dominio. Se levanta solo cuando
  arranca un proyecto que lo usa.
- **Un proyecto que usa el proxy no publica ningún puerto.** Se llega a él por su dominio, y a sus
  interfaces auxiliares por subdominios: `mail.proyecto.local`, `queue.proyecto.local`,
  `search.proyecto.local`.
- **Certificado comodín** para `*.proyecto.local`, porque los subdominios sobre HTTPS no funcionan
  con un certificado por dominio.
- **`hm tunnel <servicio>`** abre un puerto temporal contra un servicio del proyecto actual, para lo
  que no se puede enrutar por nombre. `hm tunnel db` es el caso de todos los días.
- **Es opcional por proyecto.** Quien no lo active sigue exactamente como hoy.

## Por qué no todo por el proxy

MySQL y AMQP no son HTTP: no llevan cabecera `Host`, y el nombre que se escribe al conectar sólo
sirve para resolver la IP y desaparece ahí. Traefik no puede distinguir lo que nadie le cuenta, y
lo dice él mismo cuando se le pide:

```
invalid rule: "HostSNI(`a.local`)", has HostSNI matcher, but no TLS on router
```

Se podría forzar con TLS en la base de datos y clientes que manden SNI, pero eso son certificados
para MariaDB y herramientas de escritorio configuradas a mano. Para un entorno local, el túnel bajo
demanda cuesta un comando y no cuesta nada más.

## Qué no cambia

- **Ningún proyecto existente se mueve.** Sin activar el proxy, todo funciona como hoy, con sus
  puertos publicados.
- El `hitch` que hoy termina TLS en cada proyecto sigue ahí para quien no use el proxy.
- El TLD lo elige cada proyecto: `.local` por defecto.

## Cómo se sabrá que funciona

- Dos proyectos con proxy, levantados a la vez, responden cada uno en su dominio.
- Un proyecto con proxy no publica ningún puerto en la máquina.
- Un proyecto sin proxy sigue publicando los suyos y conviviendo con el proxy.
- `hm tunnel db` da una dirección a la que TablePlus se conecta, y deja de existir al cerrarlo.
- El proxy se levanta solo al arrancar un proyecto que lo usa, y no estorba al que no.
