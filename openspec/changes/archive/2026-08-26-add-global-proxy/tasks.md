## 1. El proxy

- [x] 1.1 Generar `~/.hm/proxy/docker-compose.yml` con Traefik, como proyecto `hm-proxy`
- [x] 1.2 Crear la red compartida `hm-gateway` al levantarlo
- [x] 1.3 `hm proxy up|down|status`, con los dominios que enruta en el estado
- [x] 1.4 Exponer el panel de Traefik sólo en loopback

## 2. La superposición del proyecto

- [x] 2.1 Comprobar la versión de Compose y fallar con un mensaje claro si no admite `!reset`
- [x] 2.2 Generar `docker-compose.proxy.yml`: quitar puertos, añadir red y etiquetas, eliminar hitch
- [x] 2.3 Etiquetas de enrutado para el dominio principal contra varnish
- [x] 2.4 Subdominios para correo, colas y buscador
- [x] 2.5 Incluir el fichero en `DOCKER_COMPOSE` cuando el proyecto usa proxy
- [x] 2.6 Propiedad `USE_PROXY`, desactivada por defecto

## 3. Certificados

- [x] 3.1 Certificado comodín para `*.<dominio>` con mkcert
- [x] 3.2 Guardarlo junto al proxy y cargarlo con el proveedor de ficheros de Traefik
- [x] 3.3 Regenerarlo cuando cambia el dominio del proyecto

## 4. Ciclo de vida

- [x] 4.1 `hm start` levanta el proxy si el proyecto lo usa
- [x] 4.2 `hm stop` no lo para
- [x] 4.3 Comprobación de doctor: el proxy está en marcha y la red existe

## 5. `hm tunnel`

- [x] 5.1 Contenedor efímero en la red del proyecto, publicando un puerto libre en loopback
- [x] 5.2 Elegir puerto libre e informar de la dirección
- [x] 5.3 Cerrar y limpiar, también al interrumpir
- [x] 5.4 Fallar nombrando los servicios cuando el pedido no existe
- [x] 5.5 Entrada en la ayuda

## 6. Verificación

- [x] 6.1 Test de dos proyectos con proxy levantados a la vez, cada uno en su dominio
- [x] 6.2 Test de que un proyecto con proxy no publica ningún puerto
- [x] 6.3 Test de que uno sin proxy sigue igual y convive
- [x] 6.4 Test de los subdominios auxiliares
- [x] 6.5 Test de que activar y desactivar es reversible
- [x] 6.6 Test del túnel: alcanzable, y no queda nada al cerrarlo
- [x] 6.7 Test de la comprobación de versión de Compose
- [x] 6.8 Suite completa en mac y linux

## 7. Documentación

- [x] 7.1 `docs/proxy.md` y `docs/tunnel.md`
- [x] 7.2 Qué cambia al activarlo, y cómo volver
- [x] 7.3 Entrada en el changelog de la 1.7.0
- [x] 7.4 Marcar NET-01 y NET-02 en el backlog
