## 1. El armazón

- [x] 1.1 Declarar cada comprobación con su detección y su ejecución
- [x] 1.2 Ejecutar dentro del contenedor de PHP
- [x] 1.3 Distinguir tres estados: correcto, fallo y omitido
- [x] 1.4 Código de salida cero sólo si nada falla

## 2. Las comprobaciones

- [x] 2.1 Sintaxis con `php -l`, siempre disponible
- [x] 2.2 Estándar de código de Magento, si está instalado
- [x] 2.3 Análisis estático, si está instalado
- [x] 2.4 Formato, en seco y sin corregir
- [x] 2.5 Pruebas unitarias y compilación de DI, sólo bajo petición

## 3. Alcance

- [x] 3.1 `--changed`: ficheros que difieren de la rama base
- [x] 3.2 Sin rama base, verificar todo e indicarlo
- [x] 3.3 Pasar la lista de ficheros a las herramientas que la admiten

## 4. Salida

- [x] 4.1 Resumen legible con una línea por comprobación
- [x] 4.2 JSON con estado, recuento y salida de cada una
- [x] 4.3 Entrada en `data/command_descriptions.json`

## 5. Verificación

- [x] 5.1 Test de que un proyecto sin herramientas no falla
- [x] 5.2 Test de que un error de sintaxis se detecta
- [x] 5.3 Test de que una herramienta ausente se informa como omitida
- [x] 5.4 Test de que el código de salida distingue fallo de omisión
- [x] 5.5 Test de `--changed` sobre un repositorio con cambios
- [x] 5.6 Test de la salida JSON
- [x] 5.7 Suite completa en mac y linux

## 6. Documentación

- [x] 6.1 `docs/verify.md`
- [x] 6.2 Entrada en el changelog de la 1.7.0
- [x] 6.3 Marcar AI-01 en el backlog
