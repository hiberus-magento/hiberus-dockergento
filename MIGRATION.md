# Migración a Go — dónde estamos

> El documento que se abre al empezar una sesión. Las decisiones están en
> [docs/research/2.0-arquitectura.md](docs/research/2.0-arquitectura.md); esto es el estado.
>
> **Regla de oro**: cada comando que se porta llega con los tests del shell portados, y con el
> mismo contrato — mismos códigos de salida, mismo `--json`, mismas preguntas.

## Estado

| | |
|---|---|
| Rama | `release/2.0.0` |
| Fase | **2 · esqueleto y puente**, en marcha |
| Comandos en Go | 0 de 63 |
| El binario | `go build -o bin/hm ./cmd/hm` |
| La suite | `go test ./...` y `./tests/run.sh` |

## Cómo continuar si esta sesión se corta

```bash
git checkout release/2.0.0
go build -o bin/hm ./cmd/hm    # el binario
go test ./...                  # los tests de Go
./tests/run.sh tests/unit      # la suite de bash, la parte rápida
./bin/hm hm-go-project         # lo que la capa Go resuelve aquí
```

Lo siguiente por hacer está siempre en la primera fase sin terminar de la lista de abajo, y el
detalle de cada una en `openspec/changes/`.

## Fases

- [x] **0 · Estabilizar la 1.x.** Concurrencia, colisiones, `vendor` montado, cachés que no
  invalidaban, chequeo de memoria de la VM. Prueba de diez worktrees pasada.
- [ ] **2 · Esqueleto y puente** — *en marcha*
  - [x] Módulo, dominio, puertos y casos de uso separados de los adaptadores
  - [x] Puente a bash: todo lo no portado corre igual, con sus códigos de salida
  - [x] Resolución del proyecto en Go (raíz, worktree, properties), probada contra la de bash
  - [x] CI que compila, formatea, pasa `vet`, los tests de Go y la suite unitaria de bash
  - [ ] Distribución: `goreleaser` y binarios para darwin/linux, amd64/arm64
  - [ ] Instalación: que `hm` sea el binario y encuentre el árbol de shell
- [ ] **3 · Adaptador de Docker (SDK) + tanda 1** — donde gana todo el equipo
- [ ] **4 · Registro SQLite con las dos topologías + tanda 2**
- [ ] **5 · Servicios compartidos, seed, worktrees, GC** — donde gana el trabajo con agentes
- [ ] **6 · Adaptadores de agente: `--json`, MCP, HTTP para la web**
- [ ] **7 · Tanda 3; la 4 se queda en bash mientras compense**

Antes de la fase 5 hay una puerta: las cuatro medidas de la Fase 1 (§7 del documento de
arquitectura). Las fases 2, 3 y 4 no dependen de ellas.

## Los comandos

Tanda 1 son los del día a día; 2 los que tocan estado; 3 el resto; 4 los que orquestan
herramientas externas y pueden quedarse en shell indefinidamente.

| Comando | Área | Tanda | Implementación |
|---|---|---|---|
| `ai-context` | ai | 4 | shell |
| `ai-doctor` | ai | 4 | shell |
| `ai-init` | ai | 4 | shell |
| `ai-pull` | ai | 4 | shell |
| `ai-reset` | ai | 4 | shell |
| `bash` | tools | 3 | shell |
| `clean` | environment | 2 | shell |
| `cloud` | tools | 4 | shell |
| `cloud-login` | tools | 4 | shell |
| `compatibility` | tools | 4 | shell |
| `composer` | magento | 1 | shell |
| `config-env` | tools | 3 | shell |
| `copy-from-container` | files | 3 | shell |
| `copy-to-container` | files | 3 | shell |
| `create-project` | tools | 3 | shell |
| `db` | database | 2 | shell |
| `dbeaver` | database | 3 | shell |
| `debug-off` | tools | 3 | shell |
| `debug-on` | tools | 3 | shell |
| `describe` | environment | 1 | shell |
| `docker-compose` | tools | 3 | shell |
| `docker-stop-all` | tools | 3 | shell |
| `doctor` | environment | 1 | shell |
| `down` | environment | 2 | shell |
| `exec` | tools | 1 | shell |
| `grunt` | magento | 4 | shell |
| `install` | magento | 3 | shell |
| `launch` | environment | 3 | shell |
| `list` | environment | 1 | shell |
| `logs` | environment | 1 | shell |
| `magento` | magento | 1 | shell |
| `masquerade` | database | 3 | shell |
| `mcp` | ai | 3 | shell |
| `mysql` | database | 3 | shell |
| `mysqldump` | database | 3 | shell |
| `n98-magerun` | magento | 3 | shell |
| `npm` | magento | 3 | shell |
| `permissions` | ai | 3 | shell |
| `proxy` | environment | 2 | shell |
| `purge` | magento | 3 | shell |
| `rebuild` | environment | 3 | shell |
| `restart` | environment | 1 | shell |
| `sequelace` | database | 3 | shell |
| `set-host` | tools | 3 | shell |
| `setup` | environment | 2 | shell |
| `share` | environment | 3 | shell |
| `ssl` | tools | 4 | shell |
| `start` | environment | 1 | shell |
| `stop` | environment | 1 | shell |
| `switch` | versions | 3 | shell |
| `tableplus` | database | 3 | shell |
| `test-integration` | magento | 3 | shell |
| `test-unit` | magento | 3 | shell |
| `transfer-db` | database | 4 | shell |
| `transfer-media` | files | 4 | shell |
| `tui` | environment | 3 | shell |
| `tunnel` | environment | 3 | shell |
| `update` | versions | 3 | shell |
| `varnish-off` | tools | 3 | shell |
| `varnish-on` | tools | 3 | shell |
| `verify` | tools | 3 | shell |
| `version` | versions | 3 | shell |
| `worktree` | environment | 2 | shell |

## Cómo se porta un comando

1. Se escribe el caso de uso en `internal/app`, contra puertos, con sus tests y sin Docker.
2. Se escribe el adaptador que haga falta en `internal/adapters`.
3. Se conecta en `internal/cli` y se quita del puente.
4. Se porta el test de integración del shell, que pasa a ejecutar el binario.
5. Se marca aquí, en la tabla y en el contador de arriba.

El test `tests/unit/migration_status_test.sh` comprueba que esta tabla no miente: que están todos
los comandos y que lo marcado como Go existe de verdad en el árbol de Go.
