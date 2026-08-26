## Context

Cloudflared con túneles rápidos: un contenedor, sin cuenta, sin credenciales, sin nada instalado en
la máquina. Comprobado desde esta red antes de escribir el comando, porque era la única duda real —
ngrok ya estuvo bloqueado alguna vez y no había motivo para dar por hecho que este pasara.

Pasa: la URL pública devuelve el contenido local, `HTTP 200`.

(Una primera medición dijo lo contrario y era mía: consulté la URL antes de que el túnel registrara
sus conexiones con el borde. La conclusión de que había filtrado corporativo era falsa.)

## Goals / Non-Goals

**Goals**
- Una URL pública en un comando, sin configurar nada.
- Que quede claro que el entorno queda expuesto mientras dure.
- Que al cerrarlo no quede nada.

**Non-Goals**
- No se toca el proyecto: ni configuración, ni base de datos, ni `base_url`.
- No se gestionan túneles con nombre ni dominios propios: eso exige cuenta y credenciales
  repartidas, y es otra cosa.
- No se comparte más que HTTP. Exponer una base de datos a internet no es una funcionalidad, es un
  incidente.

## Decisions

### 1. Túnel rápido, no túnel con nombre

Sin cuenta ni credenciales que repartir por el equipo. El precio es que **la URL cambia en cada
arranque**, lo cual es correcto para una demo o para capturar unos webhooks, que es para lo que se
pide.

### 2. En primer plano, como `hm tunnel`

La vida del túnel es la vida del comando. Ctrl-C cierra y limpia. Un túnel que sigue abierto después
de cerrar la ventana es un entorno expuesto sin que nadie lo sepa, y eso no es un descuido
aceptable aquí: `--stop` existe para recogerlo, y el siguiente `hm share` limpia el anterior.

### 3. Se pide confirmación, y se dice exactamente qué se expone

No un «¿seguro?». La frase nombra el proyecto y dice que cualquiera con la URL entra, incluidos el
panel de administración y los datos que haya dentro. Es la única confirmación de esta herramienta
que protege de algo que no está en la máquina.

Con `--yes` no pregunta, para quien lo automatice a sabiendas.

### 4. La cabecera `Host` se reescribe al dominio del proyecto

Sin eso, la petición llega con el host de Cloudflare y el `nginx` del proyecto no encuentra su
vhost. Con `--http-host-header` el origen ve su propio dominio y responde normalmente.

Lo que no arregla —y se dice— es que Magento construye sus enlaces desde `base_url`, así que
apuntan al dominio local. Para webhooks es irrelevante; para una demo de navegación hay que cambiar
`base_url`, y eso se documenta en lugar de hacerlo por su cuenta: escribir en la base de datos de
alguien para una demo es peor que explicarle el comando.

### 5. Se conecta a la red del proyecto, no a la máquina

El contenedor de Cloudflared se une a la red del proyecto y apunta al servicio web por su nombre.
Así funciona igual con proyectos que publican puertos y con los que van por el proxy y no publican
ninguno.

## Risks / Trade-offs

- **Mientras está abierto, el entorno es público.** Es lo que se pide; se avisa, se confirma y se
  cierra solo al terminar el comando.
- **La URL es efímera.** Para webhooks que exijan una URL estable, hace falta un túnel con nombre y
  eso queda fuera.
- **Depende de un servicio de terceros.** Si Cloudflare no responde, el comando falla y lo dice.

## Migration Plan

Ninguna: comando nuevo.

## Open Questions

Si más adelante hace falta una URL estable, la vía es un túnel con nombre sobre un dominio de la
empresa. No se decide aquí.
