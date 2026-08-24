## 1. La derivación

- [x] 1.1 `hm_derive_project_name <directorio>`: minúsculas, sólo `[a-z0-9_-]`, recortar `-`/`_` iniciales
- [x] 1.2 Sin procesos: expansión de parámetros de Bash, nada de `awk`, `sed` ni `tr`
- [x] 1.3 Devolver vacío cuando no queda ningún carácter admisible

## 2. La resolución

- [x] 2.1 `hm_resolve_project_name`: la propiedad si tiene valor, si no el derivado de la raíz
- [x] 2.2 Derivar de la raíz del proyecto, no de `$PWD`
- [x] 2.3 Exportar el nombre resuelto antes de construir el comando de Compose
- [x] 2.4 Fallar con el código de proyecto cuando no se puede resolver ningún nombre

## 3. Que todo lo demás lo use

- [x] 3.1 Etiquetas de entorno: `hm.project` con el nombre resuelto
- [x] 3.2 Búsqueda de contenedores por servicio
- [x] 3.3 `describe` y `list`
- [x] 3.4 Comprobar que no queda ningún sitio leyendo la propiedad en crudo

## 4. `setup`

- [x] 4.1 Escribir `COMPOSE_PROJECT_NAME` sólo si difiere del derivado
- [x] 4.2 Respetar el nombre ya fijado de un proyecto existente
- [x] 4.3 No cambiar la pregunta ni su valor por defecto

## 5. Verificación

- [x] 5.1 Test unitario de la derivación contra la tabla medida de Compose
- [x] 5.2 Test de que la derivación coincide con `docker compose config` de verdad, no con la tabla
- [x] 5.3 Test de que un nombre configurado gana y no se toca
- [x] 5.4 Test de que desde un worktree el nombre derivado es el del checkout principal
- [x] 5.5 Test de que dos clones sin nombre configurado son entornos distintos
- [x] 5.6 Test de que las etiquetas dejan de escribirse vacías
- [x] 5.7 Test del directorio sin nombre válido
- [x] 5.8 Test de que `setup` no escribe la propiedad cuando se acepta el valor propuesto
- [x] 5.9 Suite completa en mac y linux

## 6. Documentación

- [x] 6.1 Documentar la resolución del nombre y la regla de derivación
- [x] 6.2 Entrada en el changelog de la 1.6.0
- [x] 6.3 Marcar ENV-01 en el backlog
