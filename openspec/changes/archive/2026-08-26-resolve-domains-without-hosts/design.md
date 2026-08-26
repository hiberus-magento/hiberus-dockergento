## Context

`hm set-host` escribe una línea en `/etc/hosts` por proyecto, y para eso pide la contraseña del
sistema. Veintitrés líneas en esta máquina, ninguna retirada nunca.

Lo comprobado antes de diseñar:

| | |
|---|---|
| `cualquiera.test` sin estar en `/etc/hosts` | resuelve a `127.0.0.1` |
| `inventado-xyz.local` sin estar en `/etc/hosts` | no resuelve |
| Cómo enruta macOS `.local` | `resolver #2 … options: mdns` |
| ¿Puede Colima publicar puertos < 1024? | sí — el proxy ya usa el 80 y el 443 |
| ¿Hay algo escuchando ya en el 53? | sí, y responde por comodín a `.test` |

Los dos primeros marcan el alcance; el último marca la actitud: **no pelear por el puerto 53**.

## Goals / Non-Goals

**Goals**
- Que un proyecto nuevo no pida la contraseña cuando no hace falta.
- Poder ofrecer la resolución a quien no la tenga ya.

**Non-Goals**
- No se toca `.local`: es imposible en macOS y no se va a fingir que sí.
- No se limpia `/etc/hosts`. Borrar líneas de un fichero del sistema que quizá alguien puso a mano
  no es algo que deba hacer una herramienta de desarrollo.
- No se sustituye a quien ya resuelva `.test`. Si Herd, DDEV o Valet ya lo hacen, se aprovecha.

## Decisions

### 1. Preguntar antes de escribir

El cambio que da todo el valor es un `if`: antes de escribir en `/etc/hosts`, comprobar si el
dominio ya resuelve a una dirección de loopback. Si resuelve, no hay nada que hacer.

Funciona con **cualquier** mecanismo que provea la resolución —el nuestro, el de Herd, el de quien
sea— porque pregunta por el resultado y no por quién lo produce. Y es inmediatamente útil en las
máquinas que ya tienen un resolvedor, sin instalar nada.

Se exige que resuelva a loopback, no simplemente que resuelva: un dominio que resuelve a una
dirección de internet es un dominio real de alguien, y ahí escribir en `/etc/hosts` es justo lo que
hay que hacer para trabajarlo en local.

### 2. Montar el resolvedor se deja fuera, a sabiendas

El plan incluía un `dnsmasq` junto al proxy respondiendo `*.<tld>`. No se construye aquí, y el
motivo es que **no se puede verificar en esta máquina**: el puerto 53 ya está ocupado por el
resolvedor que instaló otra herramienta, así que el nuestro no llegaría a arrancar ni a probarse.

Construir a ciegas algo que además pide `sudo` y puede pelearse por un puerto con un programa que
el usuario instaló a propósito es la peor combinación posible. Se documenta cómo obtener resolución
comodín —Herd, DDEV, o un `dnsmasq` a mano— y se deja el servicio propio para cuando haya dónde
probarlo.

Lo que sí se construye funciona igual venga la resolución de donde venga, porque pregunta por el
resultado y no por quién lo produce. En una máquina con resolvedor, el beneficio es inmediato y sin
instalar nada.

### 3. El fichero de `/etc/resolver` se escribe una vez, y se pide permiso

En macOS un TLD se dirige a un servidor DNS con un fichero en `/etc/resolver/<tld>`, y eso exige
`sudo`. Una vez, no una por proyecto — que es exactamente el trato que se quería mejorar.

Se pide explicando qué se va a escribir y por qué. En Linux la ruta es distinta y depende del
gestor de red, así que ahí se explica qué hacer en lugar de tocar la configuración del sistema a
ciegas.

### 4. `.local` no se toca

macOS enruta `.local` a mDNS, punto. Un servidor DNS no ve esas consultas, y los dominios `.local`
funcionan hoy porque `/etc/hosts` se consulta antes.

Esto no se intenta arreglar ni se advierte cada vez: un proyecto en `.local` sigue exactamente como
está. Quien quiera dejar de escribir en `/etc/hosts` cambia el TLD de su proyecto, que ya es una
decisión suya (D8).

## Risks / Trade-offs

- **Depende de que algo resuelva el TLD.** Si el resolvedor desaparece —se desinstala Herd, se para
  el proxy— los dominios dejan de resolver y el proyecto parece caído. `hm doctor` lo señala, que
  es la diferencia entre un misterio y un aviso.
- **`/etc/hosts` gana precedencia sobre el DNS**, así que un proyecto con línea antigua seguirá
  usándola aunque el DNS también responda. No es un problema: apuntan al mismo sitio.
- **Un `sudo` sigue existiendo**, la primera vez. A cambio de que no haya uno por proyecto.

## Migration Plan

Ninguna. Los proyectos existentes conservan sus líneas y su comportamiento.

## Open Questions

Ninguna.
