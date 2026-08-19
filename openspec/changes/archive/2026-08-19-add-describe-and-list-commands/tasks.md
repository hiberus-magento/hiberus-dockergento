## 1. Recolección de datos

- [x] 1.1 Crear una tarea compartida en `console/tasks/` que reúna los datos del proyecto actual desde `config/docker/properties.json`, `data/requirements.json` y `composer.lock`
- [x] 1.2 Añadir la obtención de estado y puertos de los contenedores mediante el descubrimiento por etiquetas
- [x] 1.3 Resolver el estado de Xdebug y el modo de despliegue sin necesidad de entrar en el contenedor cuando sea posible
- [x] 1.4 Medir el tiempo total de recolección y anotar si hace falta cachear

## 2. Esquema

- [x] 2.1 Definir el esquema JSON de `describe` con los bloques `project`, `magento`, `services`, `paths`, `tooling` y `credentials`
- [x] 2.2 Definir el esquema JSON de `list`
- [x] 2.3 Documentar qué claves son estables y cuáles pueden cambiar sin subir `schema_version`

## 3. Comando `hm describe`

- [x] 3.1 Crear `console/commands/describe.sh` con soporte de `--json` y `--with-secrets`
- [x] 3.2 Implementar la salida de texto agrupada por bloques, con URLs y estado en primer lugar
- [x] 3.3 Omitir credenciales salvo `--with-secrets`
- [x] 3.4 Devolver el código de proyecto no configurado fuera de un proyecto
- [x] 3.5 Funcionar con el entorno parado

## 4. Comando `hm list`

- [x] 4.1 Crear `console/commands/list.sh` con soporte de `--json`
- [x] 4.2 Hacer que no dependa del directorio actual, incluyendo su exclusión de las validaciones de proyecto de `bin/run`
- [x] 4.3 Agrupar por proyecto y distinguir checkout principal de worktrees
- [x] 4.4 Marcar entornos huérfanos y entornos sin metadatos
- [x] 4.5 Mensaje útil cuando no hay ningún entorno

## 5. Verificación

- [x] 5.1 Comprobar `describe` en un proyecto levantado, en uno parado y fuera de un proyecto
- [x] 5.2 Comprobar que la salida por defecto no contiene credenciales y que `--with-secrets` sí
- [x] 5.3 Comprobar `list` con dos proyectos simultáneos, uno de ellos parado
- [x] 5.4 Validar ambas salidas JSON con `jq -e .` y contra el esquema documentado
- [x] 5.5 Verificar en mac y en linux, comprobando que `paths` refleja la estrategia de montaje de cada plataforma

## 6. Documentación

- [x] 6.1 Crear `docs/describe.md` y `docs/list.md`
- [x] 6.2 Añadir ambos comandos a `data/command_descriptions.json`
- [x] 6.3 Publicar el esquema JSON en la documentación
- [x] 6.4 Marcar CLI-02 y CLI-03 como `spec` en `docs/research/backlog.md` con enlace a este cambio
