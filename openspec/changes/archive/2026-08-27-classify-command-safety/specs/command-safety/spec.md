## ADDED Requirements

### Requirement: Cada comando declara su riesgo

Todo comando SHALL declarar su nivel de riesgo junto al resto de sus metadatos.

#### Scenario: Todos declarados
- **WHEN** se revisan los comandos de la herramienta
- **THEN** cada uno tiene un nivel de riesgo declarado

#### Scenario: Niveles admitidos
- **WHEN** un comando declara su riesgo
- **THEN** es uno de los tres niveles previstos: sin efectos, reversible, o destructivo

#### Scenario: Un comando nuevo sin declarar
- **WHEN** se añade un comando y se olvida declarar su riesgo
- **THEN** la comprobación automática lo detecta

### Requirement: Las clasificaciones internas no divergen

Las listas internas que la herramienta usa para decidir comportamiento SHALL coincidir con la
clasificación declarada.

#### Scenario: Lo que se rechaza desde un worktree
- **WHEN** un comando está en la lista de los que alteran el entorno
- **THEN** su riesgo declarado no es «sin efectos»

#### Scenario: Lo declarado como destructivo
- **WHEN** un comando se declara destructivo
- **THEN** o bien altera el entorno, o bien alcanza más allá del proyecto

### Requirement: Generar la configuración de permisos

`hm permissions` SHALL producir una configuración de permisos derivada de la clasificación.

#### Scenario: Lo seguro se permite
- **WHEN** se genera la configuración
- **THEN** los comandos sin efectos aparecen permitidos

#### Scenario: Lo destructivo se confirma
- **WHEN** se genera la configuración
- **THEN** los comandos destructivos aparecen como que requieren confirmación

#### Scenario: Modo estricto
- **WHEN** se pide la configuración más restrictiva
- **THEN** sólo los comandos sin efectos quedan permitidos

#### Scenario: No se escribe nada
- **WHEN** se genera la configuración
- **THEN** se muestra por salida y no se modifica ningún fichero de configuración

#### Scenario: Para una máquina
- **WHEN** la salida se canaliza o se pide JSON
- **THEN** la configuración sale en un formato que la plataforma puede consumir directamente
