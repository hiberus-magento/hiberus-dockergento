# Dejar de pedir la contraseña para cada dominio

## Por qué

Cada proyecto nuevo pide la contraseña del sistema para escribir una línea en `/etc/hosts`:

```
Your system password is needed to add an entry to /etc/hosts...
```

En esta máquina hay **23 líneas** puestas así, una por proyecto, y ninguna se retira cuando el
proyecto desaparece. Es un fichero del sistema que va acumulando restos de trabajos terminados.

## Qué lo hace innecesario

Un resolvedor comodín: si `*.test` resuelve a `127.0.0.1`, cualquier dominio bajo ese TLD funciona
sin escribir nada, y con el proxy escuchando ahí ya está todo hecho.

Y resulta que **en esta máquina ya funciona**: `cualquiera.test` y `proyecto-inventado.test`
resuelven a `127.0.0.1` sin aparecer en `/etc/hosts`, cortesía de una herramienta ya instalada.
El trabajo no es montar un resolvedor: es **darse cuenta de cuándo ya no hace falta escribir**.

## Qué cambia

- **Antes de tocar `/etc/hosts` se comprueba si el dominio ya resuelve** a una dirección local. Si
  resuelve, no se escribe nada y no se pide contraseña.
- Se documenta cómo conseguir resolución comodín para quien no la tenga. **Montar un servidor DNS
  propio se deja fuera de este cambio**: en esta máquina el puerto 53 ya está ocupado por otro
  resolvedor, así que no habría dónde probarlo, y construir a ciegas algo que pide `sudo` y puede
  disputar un puerto no compensa.
- `hm doctor` dice cuál de las dos cosas está pasando.

## El límite, que hay que decir

**Sólo sirve para TLD que no sean `.local`.** macOS enruta `.local` a mDNS/Bonjour, no a DNS:

```
resolver #2
  domain   : local
  options  : mdns
```

Un servidor DNS nunca llega a ver esas consultas. Los dominios `.local` de hoy funcionan
precisamente porque `/etc/hosts` se consulta antes que cualquier resolución, y seguirán haciéndolo
igual.

Así que esto beneficia a los proyectos que elijan `.test` (decisión D8: el TLD lo elige cada
proyecto). Para los demás, todo sigue como está.

## Qué no cambia

- Ningún proyecto existente se toca, ni se le quita su línea de `/etc/hosts`.
- Quien use `.local` sigue con el mismo flujo de siempre, contraseña incluida.
- No se toca `/etc/hosts` para retirar nada: lo que está escrito, escrito se queda.

## Cómo se sabrá que funciona

- Un proyecto en un TLD que ya resuelve no pide la contraseña ni escribe en `/etc/hosts`.
- Un proyecto en `.local` se comporta exactamente como antes.
- Con el resolvedor del proxy activo, un dominio inventado bajo ese TLD resuelve sin haberlo
  registrado en ningún sitio.
