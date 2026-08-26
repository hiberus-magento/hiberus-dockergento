## ADDED Requirements

### Requirement: Destruir volúmenes exige una respuesta

`hm down` con la opción de borrar volúmenes SHALL pedir confirmación antes de destruir nada.

#### Scenario: Se enumera lo que se va a perder
- **WHEN** se pide destruir el entorno con sus volúmenes de forma interactiva
- **THEN** se nombran los volúmenes que se van a borrar antes de preguntar

#### Scenario: No confirmar no destruye
- **WHEN** no se confirma
- **THEN** los volúmenes siguen existiendo y el entorno queda como estaba

#### Scenario: Se ofrece guardar antes
- **WHEN** se pide destruir el entorno con sus volúmenes
- **THEN** se ofrece guardar una copia de la base de datos como primera opción

#### Scenario: Guardar y destruir
- **WHEN** se acepta guardar
- **THEN** queda una copia restaurable y sólo después se destruye el entorno

#### Scenario: Destruir sin guardar
- **WHEN** se elige destruir sin guardar
- **THEN** no se crea ninguna copia y el entorno se destruye

#### Scenario: Sin volúmenes no se pregunta
- **WHEN** se destruye el entorno sin pedir borrar volúmenes
- **THEN** no se pregunta nada, porque no se pierde nada

#### Scenario: Sin nadie a quien preguntar
- **WHEN** no hay terminal, o se ha indicado que no se pregunte
- **THEN** se destruye sin preguntar, como hasta ahora

### Requirement: Guardar al parar

`hm stop` SHALL poder guardar una copia de la base de datos antes de parar el entorno.

#### Scenario: Se pide la copia
- **WHEN** se para el entorno indicando que se quiere una copia
- **THEN** la copia queda guardada antes de parar los servicios

#### Scenario: Parar no guarda por su cuenta
- **WHEN** se para el entorno sin indicar nada
- **THEN** no se crea ninguna copia y el comando se comporta como siempre

#### Scenario: Si la copia falla no se para
- **WHEN** la copia no se puede crear
- **THEN** el entorno no se para, para no dejar creer que hay una copia que no existe

### Requirement: Parar toda la máquina exige una respuesta

El comando que para todos los contenedores de la máquina SHALL pedir confirmación e informar de su
alcance.

#### Scenario: Se dice cuántos y de quién
- **WHEN** se pide parar todos los contenedores de forma interactiva
- **THEN** se indica cuántos se van a parar y cuántos no pertenecen al proyecto actual

#### Scenario: No confirmar no para nada
- **WHEN** no se confirma
- **THEN** ningún contenedor se detiene

#### Scenario: Nada que parar
- **WHEN** no hay contenedores en marcha
- **THEN** se informa y no se pregunta

#### Scenario: Sin nadie a quien preguntar
- **WHEN** no hay terminal, o se ha indicado que no se pregunte
- **THEN** se paran sin preguntar
