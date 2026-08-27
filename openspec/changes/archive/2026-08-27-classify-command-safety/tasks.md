## 1. La clasificación

- [x] 1.1 Campo de riesgo en cada comando de `data/command_descriptions.json`
- [x] 1.2 Clasificar los sesenta comandos, con criterio documentado
- [x] 1.3 Los que envuelven a otros comandos, por su peor uso razonable

## 2. `hm permissions`

- [x] 2.1 Generar la configuración a partir de la clasificación
- [x] 2.2 Permitir lo seguro y lo reversible; confirmar lo destructivo
- [x] 2.3 `--strict`: permitir sólo lo seguro
- [x] 2.4 No escribir en ningún fichero
- [x] 2.5 Entrada en `data/command_descriptions.json`

## 3. Que no diverjan

- [x] 3.1 Test: todo comando tiene riesgo declarado y es un nivel válido
- [x] 3.2 Test: lo que altera el entorno no está declarado como seguro
- [x] 3.3 Test: lo declarado destructivo altera el entorno o sale del proyecto
- [x] 3.4 Test: todo comando con fichero tiene entrada, y al revés

## 4. Verificación

- [x] 4.1 Test de que la configuración permite consultar y confirma destruir
- [x] 4.2 Test del modo estricto
- [x] 4.3 Test de que no se escribe nada
- [x] 4.4 Suite completa en mac y linux

## 5. Documentación

- [x] 5.1 `docs/permissions.md`: los tres niveles y qué significa cada uno
- [x] 5.2 Entrada en el changelog de la 1.7.0
- [x] 5.3 Marcar AI-02 en el backlog
