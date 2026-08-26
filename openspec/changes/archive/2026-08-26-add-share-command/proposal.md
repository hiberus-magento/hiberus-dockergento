# Enseñar el entorno local desde fuera

## Por qué

Dos necesidades reales que hoy no tienen respuesta:

**Enseñar avances sin desplegar.** Un cliente o alguien de QA quiere ver cómo va algo. La única
salida es desplegar a un entorno compartido, con lo que eso arrastra: esperar, pisar el trabajo de
otro, o montar un entorno para una demo de diez minutos.

**Recibir webhooks de verdad.** Pasarelas de pago, ERPs y marketplaces envían peticiones a una URL
pública. Contra un entorno local no llegan, así que se prueba a ciegas o en un entorno compartido
que no es el que se está tocando.

## Qué cambia

- **`hm share`** abre un túnel y devuelve una URL pública temporal que apunta a este proyecto.
- **`hm share --stop`** lo cierra. Mientras vive, el comando se queda en primer plano.
- Antes de abrirlo **avisa de lo que implica y pide confirmación**: mientras esté abierto, cualquiera
  con la URL entra en el entorno.

## Cómo

Con **Cloudflared** y sus túneles rápidos: sin cuenta, sin credenciales y sin instalar nada en la
máquina — se ejecuta en un contenedor. Verificado de extremo a extremo desde esta red, que era la
duda: la URL pública devuelve el contenido local.

Se decidió Cloudflared y no ngrok porque ngrok ya ha estado bloqueado en la empresa (decisión D3).

## Qué no cambia

- El proyecto no se modifica de ninguna forma. Ni su configuración, ni su base de datos.
- No hace falta el proxy: funciona igual con proyectos que publican puertos y con los que no.

## Lo que hay que decir en voz alta

**La URL cambia en cada arranque.** Los túneles rápidos son anónimos y efímeros. Para una URL
estable hacen falta una cuenta de Cloudflare y un dominio propio, y eso queda fuera de aquí.

**Los enlaces absolutos de Magento apuntan al dominio local.** Magento construye sus URLs desde
`base_url`, así que las páginas se ven pero sus enlaces llevan al dominio de tu máquina. Para una
demo de navegación hay que cambiar `base_url`; para recibir webhooks da igual, que es el caso donde
más falta hace.

## Cómo se sabrá que funciona

- La URL pública devuelve lo que sirve el proyecto.
- Al cerrar, la URL deja de responder y no queda nada corriendo.
- Sin confirmar, no se abre nada.
