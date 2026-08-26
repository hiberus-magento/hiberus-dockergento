## 1. Preguntar antes de escribir

- [x] 1.1 Resolver el dominio sin usar el fichero de hosts como única fuente
- [x] 1.2 Considerar suficiente sólo una respuesta de loopback
- [x] 1.3 Saltar la escritura y el `sudo` cuando ya resuelve
- [x] 1.4 Mantener el comportamiento de siempre cuando no resuelve

## 2. El resolvedor: fuera de alcance, a sabiendas

- [x] 2.1 ~~Servicio de DNS junto al proxy~~ — no verificable aquí: el puerto 53 ya está ocupado
- [x] 2.2 ~~No levantarlo si el puerto está ocupado~~ — sin servicio, no aplica
- [x] 2.3 ~~Escribir `/etc/resolver/<tld>`~~ — pide `sudo` y no se puede probar; se documenta
- [x] 2.4 Documentar cómo obtener resolución comodín con lo que ya existe

## 3. Diagnóstico

- [x] 3.1 Comprobación: el dominio del proyecto resuelve, y por qué vía
- [x] 3.2 Señalar cuándo no resuelve de ninguna forma

## 4. Verificación

- [x] 4.1 Test de que un dominio ya resoluble no toca el fichero de hosts
- [x] 4.2 Test de que uno que no resuelve sí lo toca
- [x] 4.3 Test de que un dominio que resuelve fuera de loopback sí lo toca
- [x] 4.4 ~~Test del resolvedor~~ — sin servicio propio
- [x] 4.5 ~~Test del puerto ocupado~~ — sin servicio propio
- [x] 4.6 Suite completa en mac y linux

## 5. Documentación

- [x] 5.1 `docs/dns.md`: qué resuelve qué, y por qué `.local` no entra
- [x] 5.2 Entrada en el changelog de la 1.7.0
- [x] 5.3 Marcar NET-03 en el backlog
