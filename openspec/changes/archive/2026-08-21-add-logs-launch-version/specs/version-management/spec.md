## ADDED Requirements

### Requirement: Versión como comando

`hm version` SHALL informar de la versión de la herramienta y de las del entorno de contenedores
en el que trabaja.

#### Scenario: Lo que informa
- **WHEN** se ejecuta el comando
- **THEN** informa de la versión de la CLI con su referencia exacta, de la versión de Docker y de
  la de Compose

#### Scenario: Fuera de un proyecto
- **WHEN** se ejecuta en un directorio que no es un proyecto
- **THEN** responde igualmente: sirve para informar de un problema, y el problema puede ser que
  no haya proyecto

#### Scenario: Docker no disponible
- **WHEN** Docker no está instalado o no responde
- **THEN** informa de la versión de la CLI y deja constancia de que las otras no se han podido
  determinar, sin fallar

#### Scenario: Los dos formatos
- **WHEN** la salida se canaliza o se pide JSON
- **THEN** la respuesta es el sobre JSON habitual con los mismos datos

#### Scenario: La bandera sigue igual
- **WHEN** se usa `hm --version`
- **THEN** su salida no cambia respecto a la de antes de este cambio
