## Context

Un agente que usa esta herramienta necesita que alguien decida qué puede ejecutar sin preguntar. En
la máquina de referencia esa decisión está tomada así: `Bash`, todo permitido. Con lo que
`hm down -v` es tan libre como `hm describe`.

No es descuido: mantener a mano una lista de sesenta comandos, y actualizarla cada vez que aparece
uno, no lo hace nadie.

Y hay dos listas escritas a mano ya dentro de la herramienta —`hm_alters_environment` y
`hm_creates_containers`—, cada una en su fichero. Una tercera sería el momento en que empiezan a
contradecirse.

## Goals / Non-Goals

**Goals**
- Una sola declaración de riesgo por comando, en el sitio por el que ya hay que pasar.
- Que generar la configuración de permisos deje de ser trabajo manual.
- Que las clasificaciones existentes no puedan divergir sin que se note.

**Non-Goals**
- No se escribe en la configuración de nadie. Se imprime; aplicarla es decisión suya.
- No se sustituyen las funciones internas por lectura de JSON: están en el camino caliente.
- No se inventa un modelo de permisos propio. Se genera el formato que la plataforma ya entiende.

## Decisions

### 1. El riesgo se declara donde ya se declara todo lo demás

En `data/command_descriptions.json`, junto a la descripción, el uso y el grupo. Es el fichero por
el que **hay que pasar obligatoriamente** para añadir un comando —sin entrada ahí, el comando no
sale en la ayuda—, así que es el único sitio donde una clasificación nueva no se olvida.

### 2. Tres niveles, porque dos no distinguen lo que importa

| Nivel | Qué significa | Ejemplos |
|---|---|---|
| `safe` | No cambia nada. Se puede ejecutar mil veces | `describe`, `list`, `doctor`, `logs`, `verify` |
| `caution` | Cambia cosas, y se puede deshacer | `start`, `magento`, `composer`, `setup` |
| `dangerous` | Destruye datos, o alcanza más allá del proyecto | `down`, `docker-stop-all`, `db restore`, `share` |

Con dos niveles habría que elegir entre tratar `hm start` como `hm down -v` —y entonces el agente
pregunta por todo y nadie lee las preguntas— o como `hm describe`, y entonces no protege de nada.

`share` está en el tercero aunque no borre nada: expone el entorno a internet, y eso es lo que
menos se puede deshacer de todo.

### 3. Las funciones internas siguen siendo rápidas, pero se comprueban

`hm_alters_environment` y `hm_creates_containers` se ejecutan en **cada invocación**. Leerlas de un
JSON costaría una llamada a `jq` —unos 36 ms— en el camino que precisamente se optimizó hasta los
0,4 segundos.

Así que siguen siendo listas en Bash, y una prueba comprueba que **coinciden con la clasificación
declarada**. La fuente de verdad para las personas es una; la rápida es una copia vigilada. Si
alguien añade un comando destructivo y no lo declara, la prueba lo dice.

### 4. Generar, no escribir

`hm permissions` imprime la configuración. No busca ficheros, no los modifica, no adivina qué
plataforma se usa. Quien la quiera, la copia.

Escribir en el fichero de configuración de alguien —que puede tener reglas propias, comentarios y
un orden que le importa— para ahorrarle un copiar y pegar no es un buen trato.

### 5. Por defecto permite trabajar; `--strict` permite mirar

La configuración normal permite `safe` y `caution`, y exige confirmación para `dangerous`. Es la
que deja a un agente hacer su trabajo sin poder destruir.

`--strict` permite sólo `safe`. Es para un agente que únicamente diagnostica, o para quien prefiera
autorizar cada cambio.

## Risks / Trade-offs

- **La clasificación es un juicio.** `hm magento` puede ejecutar `setup:upgrade`, que es destructivo,
  y está en `caution` porque también ejecuta `cache:clean`. Los comandos que envuelven a otros no se
  pueden clasificar mejor que por su peor uso razonable, y se documenta.
- **Dos listas siguen existiendo**, aunque vigiladas. La alternativa era pagar `jq` en cada
  invocación.
- **La configuración generada puede quedarse anticuada** en el fichero de quien la copió. Volver a
  generarla es un comando.

## Migration Plan

Ninguna: comando nuevo y un campo más en un fichero de datos.

## Open Questions

Otros formatos —Cursor, Copilot— cuando alguien los use. El generador está preparado para más de
uno, pero inventar formatos que nadie consume no ayuda a nadie.
