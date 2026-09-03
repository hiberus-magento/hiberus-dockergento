# global-proxy Specification

## Purpose
TBD - created by archiving change add-global-proxy. Update Purpose after archive.
## Requirements
### Requirement: Varios proyectos a la vez

Los proyectos que usen el proxy SHALL poder estar levantados simultáneamente, cada uno accesible por
su dominio.

#### Scenario: Dos proyectos levantados
- **WHEN** dos proyectos con proxy están en marcha al mismo tiempo
- **THEN** cada uno responde en su propio dominio, sin que ninguno impida arrancar al otro

#### Scenario: Sin puertos publicados
- **WHEN** un proyecto usa el proxy
- **THEN** ninguno de sus servicios publica puertos en la máquina

#### Scenario: Convivencia con quien no lo usa
- **WHEN** un proyecto sin proxy está levantado junto a uno con proxy
- **THEN** el primero sigue publicando sus puertos y funcionando como siempre

### Requirement: Activarlo es una propiedad del proyecto

El uso del proxy SHALL ser una preferencia de cada proyecto, desactivada por defecto.

#### Scenario: Por defecto no se usa
- **WHEN** un proyecto no expresa ninguna preferencia
- **THEN** publica sus puertos y no depende del proxy, igual que antes de que existiera

#### Scenario: Activar
- **WHEN** se activa en las propiedades del proyecto y se regenera la configuración
- **THEN** el proyecto pasa a enrutarse por el proxy y deja de publicar puertos

#### Scenario: Desactivar
- **WHEN** se desactiva y se regenera
- **THEN** el proyecto vuelve a publicar sus puertos

#### Scenario: Una versión de Compose que no lo admite
- **WHEN** se activa el proxy con una versión de Docker Compose que no soporta la superposición
  necesaria
- **THEN** el comando falla explicando qué versión hace falta, en lugar de generar algo que no
  funciona

### Requirement: Las interfaces auxiliares tienen su subdominio

Las interfaces web auxiliares de un proyecto con proxy SHALL ser accesibles en subdominios de su dominio.

#### Scenario: Correo, colas y buscador
- **WHEN** un proyecto con proxy publica interfaces web auxiliares
- **THEN** cada una es accesible en un subdominio propio del dominio del proyecto

#### Scenario: Certificado comodín
- **WHEN** se accede por HTTPS a un subdominio del proyecto
- **THEN** el certificado lo cubre, porque cubre todo el comodín del dominio

### Requirement: El proxy se levanta cuando hace falta

El proxy SHALL estar en marcha cuando un proyecto que lo usa esté levantado, sin que nadie tenga que arrancarlo a mano.

#### Scenario: Al arrancar un proyecto que lo usa
- **WHEN** se arranca un proyecto con proxy y el proxy no está en marcha
- **THEN** el proxy se levanta antes de que el proyecto quede listo

#### Scenario: Al arrancar uno que no lo usa
- **WHEN** se arranca un proyecto sin proxy
- **THEN** el proxy no se levanta

#### Scenario: Parar el proyecto no para el proxy
- **WHEN** se para un proyecto que usa el proxy
- **THEN** el proxy sigue en marcha, porque otros proyectos pueden depender de él

#### Scenario: Control manual
- **WHEN** se pide levantar, parar o consultar el proxy
- **THEN** el comando lo hace e informa de su estado y de qué dominios enruta

### Requirement: Lo que no es HTTP se alcanza bajo demanda

`hm tunnel` SHALL abrir un acceso temporal desde la máquina a un servicio del proyecto actual.

#### Scenario: Abrir un túnel
- **WHEN** se pide un túnel a un servicio del proyecto
- **THEN** se indica una dirección local desde la que ese servicio es alcanzable

#### Scenario: Varios a la vez
- **WHEN** se abren túneles a servicios de proyectos distintos
- **THEN** cada uno recibe una dirección propia y ninguno impide al otro

#### Scenario: Cerrar
- **WHEN** se cierra el túnel
- **THEN** la dirección deja de estar disponible y no queda nada corriendo

#### Scenario: Un servicio que no existe
- **WHEN** se pide un túnel a un servicio que el proyecto no tiene
- **THEN** el comando falla nombrando los servicios disponibles

### Requirement: The proxy is one thing, whichever half starts it

The tool SHALL generate the same proxy configuration from either implementation.

#### Scenario: Started by one and stopped by the other

- **WHEN** the proxy is started and then stopped through different halves of the tool
- **THEN** both act on the same compose project, and it is stopped

#### Scenario: The configuration has not changed

- **WHEN** the proxy is started again
- **THEN** its compose file is left alone rather than rewritten

### Requirement: What is in the way is named

The tool SHALL name the container holding the ports the proxy needs, and SHALL name the same one
however it is asked.

#### Scenario: A project that does not use the proxy is up

- **WHEN** the proxy is started while another environment publishes port 80 or 443
- **THEN** it is refused, and the container holding it is named

#### Scenario: One container holds 80 and another holds 443

- **WHEN** both ports are held by different containers
- **THEN** the one holding 80 is the one named

### Requirement: What is routed is what the router says

The tool SHALL report the proxy's routes as the proxy itself reports them.

#### Scenario: A router that never came up

- **WHEN** a container carries routing labels but the router failed
- **THEN** it is not reported as routed
