## Context

Comprobar un cambio en un proyecto de Magento hoy exige saber qué tiene ese proyecto instalado y
con qué configuración. Con agentes de por medio eso se paga muchas veces al día, y lo que no es
un solo comando no se ejecuta.

El dato que decide el diseño se midió sobre los catorce proyectos de esta máquina: **las
herramientas no son las mismas en ninguno**. Diez tienen PHPUnit, seis PHPStan, cinco el estándar
de Magento, tres no tienen nada. Una lista fija de comprobaciones fallaría casi siempre.

## Goals / Non-Goals

**Goals**
- Un comando que responda «esto está bien» o «esto no» sin configurar nada.
- Que sirva igual a una persona, a un agente al cerrar una tarea y a un trabajo de CI.
- Que lo que falte se note como ausente, no como error.

**Non-Goals**
- No instala herramientas ni toca `composer.json`. Lo que el proyecto tenga es lo que hay.
- No corrige. Un comando que arregla lo que encuentra es un comando que hace cambios que nadie ha
  leído, y con agentes de por medio eso es exactamente lo que no se quiere.
- No sustituye la revisión humana ni la de CI: es lo que se ejecuta antes de pedirlas.

## Decisions

### 1. Se descubre lo que hay, y ausente no es fallo

Cada comprobación se declara con cómo detectar si es aplicable y cómo ejecutarla. Antes de nada, se
mira si su binario existe en el `vendor/bin` del proyecto.

Lo ausente se informa como **omitido**, con el motivo. Es la diferencia entre «este proyecto no
tiene PHPStan» y «PHPStan ha fallado», que llevan a acciones opuestas.

### 2. Todo corre dentro del contenedor

Las herramientas viven en el `vendor` del proyecto y necesitan la versión de PHP del proyecto. En
la máquina puede no haber PHP, o haber otro. Se ejecutan con `exec` en el contenedor de PHP, que es
donde ya se ejecuta todo lo demás.

### 3. Rápido por defecto, lento a petición

`php -l`, PHPCS, PHPStan y PHP-CS-Fixer en seco son segundos. Las pruebas unitarias son minutos y
la compilación de DI puede ser mucho más.

Un comando que tarda cinco minutos no se ejecuta al terminar cada cambio, y el que no se ejecuta
no sirve de nada. Así que por defecto van las rápidas, y `--all` añade las otras.

### 4. La comprobación de sintaxis siempre está

`php -l` no necesita que el proyecto tenga nada instalado. Es la única que se puede garantizar, y
detecta lo que más duele: un fichero que rompe el sitio entero. En un proyecto sin herramientas,
`hm verify` sigue sirviendo para algo.

### 5. `--changed` compara contra la rama base, no contra el último commit

Al cerrar una tarea, lo que importa es todo lo que ha cambiado en la rama, no lo del último commit.
Se usa el punto de divergencia con la rama principal.

Si no se puede determinar —sin git, o sin rama base— se revisa todo y se dice, en lugar de revisar
sólo una parte creyendo que es el total.

### 6. Una salida que distingue el resultado del ruido

Cada comprobación aporta estado, cuántos problemas ha encontrado y su salida cruda. En texto se
resume; en JSON va completo, porque quien lo consume por ahí puede querer el detalle.

El código de salida es cero sólo si nada ha fallado. Omitido no es fallo.

## Risks / Trade-offs

- **Dos proyectos dan resultados distintos** con el mismo comando, porque tienen herramientas
  distintas. Es deliberado y se muestra: la alternativa es fallar en los que no tienen.
- **No corrige**, así que hay un paso manual después. Es intencionado.
- **La configuración de cada herramienta es la del proyecto.** Si el `phpstan.neon` de alguien está
  mal, esto lo hereda. No es sitio para imponer una configuración común.

## Migration Plan

Ninguna: comando nuevo.

## Open Questions

Si conviene una configuración común para el departamento —un `phpstan.neon` de referencia— es una
conversación aparte, y este comando la aprovecharía sin cambios.
