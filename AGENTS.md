# TodoMark — AGENTS.md

## Instrucciones Permanentes (aplican SIEMPRE, sin que el usuario lo pida)

### 1. Plan de trabajo (`PLAN.md`)
- Al iniciar cualquier sesión o tarea nueva: **leer `PLAN.md` completo antes de tocar código**.
- Ejecutar las fases/tareas **en orden, una a la vez**. No avanzar a la siguiente hasta cerrar y verificar la actual.
- Mantener una lista de todos con el estado real; marcar una tarea completada SOLO tras pasar la verificación (§4).
- Si la implementación se desvía de lo previsto en `PLAN.md`, proponer actualizar el plan, no improvisar en silencio.

### 2. Minimalismo
- El cambio mínimo que resuelve el problema. Sin refactors de regalo, ni reescrituras, ni "mejoras" no pedidas.
- Sin nuevas dependencias sin permiso explícito del usuario.
- Reutilizar componentes, servicios y patrones existentes antes de crear nuevos.
- Sin logs de debug, comentarios narrativos ni código muerto dejado atrás.

### 3. UI: no deformar
- **No alterar layout, espaciados, tipografía, colores ni estructura de componentes existentes** salvo que la tarea lo pida explícitamente.
- Elementos nuevos deben seguir el design system existente: tokens de `web/src/styles/theme.css` y la skill `todomark-design` (ver «Diseño / UI» más abajo). Nunca colores/fuentes/espaciados sueltos fuera de esos tokens.
- No mover rutas, no cambiar textos visibles, no reorganizar páginas de paso.
- Si un fix requiere tocar UI: cambio más pequeño posible y mencionarlo en el resumen final.

### 4. Verificación obligatoria (Definition of Done)
Una tarea no está terminada sin pasar TODOS los checks que apliquen:

| Servicio      | Verificación |
|---------------|--------------|
| `web/`        | `npx tsc --noEmit` + `npm run build` (Vite NO tipa por sí solo) |
| `api-go/`     | `go build ./...` |

`evolution` es la imagen oficial `evoapicloud/evolution-api` (Docker Hub), no hay código propio que compilar ahí.

- Tests: si el servicio tiene script `test`, ejecutarlo. Si no hay infra de tests todavía, tsc + build es el mínimo; avisar al usuario en lugar de saltarse el paso en silencio.
- Arreglar todo error que introduzcas. **Nunca dejar el build roto.**

### 5. Docker (el proyecto corre en Docker, NO en dev servers locales)
- Verificación end-to-end SIEMPRE vía Docker Compose:

  ```bash
  docker compose up -d --build         # reconstruir y levantar en background
  docker compose ps                    # los 4 servicios deben estar Up
  docker compose logs -f todamark-web  # o todomark-api / evolution / postgres
  ```

- Tras cambios de código NO basta `restart`: hay que `up -d --build <servicio>`.
- Servicios y puertos: `todamark-web` :8080 (nombre del contenedor así, con "a", no `todomark-web`), `todomark-api` :8081 (Go, `api-go/`), `evolution` :3100 (evoapicloud/evolution-api), `postgres` (sin puerto expuesto, solo red interna).
- `VITE_API_URL` se pasa como build ARG en `docker-compose.yml` (no env de runtime): si cambia, reconstruir `todamark-web`.
- `vite dev` / `go run .` locales solo para iterar rápido; la validación final es en Docker.
- Al cerrar una tarea que afecta a un servicio: reconstruir ese servicio y confirmar que sus logs no muestran errores.

### 6. Ejecución decisiva (anti-bucle)
- **Máximo 1 lectura por archivo** en una misma tarea. Si necesitas revisar algo ya leído, usa una búsqueda puntual con contexto, nunca re-leer el archivo entero.
- Planificar **solo el paso inmediato**: editar → verificar → siguiente. No re-planificar lo ya decidido.
- Ante dos opciones viables: elegir la más simple y ejecutar, o preguntar al usuario UNA sola vez. Nunca deliberar el mismo punto dos veces.
- Si detectas que repites el mismo análisis en el razonamiento, **corta y ejecuta** el cambio más pequeño que desbloquee la tarea.
- Una tarea nunca termina sin una acción: si no hay edits ni comandos ejecutados, el turno está incompleto.

### 7. Cierre obligatorio de respuesta
Toda respuesta que haya tocado código o haya ejecutado una tarea termina SIEMPRE con una sección:

```
## Pendientes
- [ ] <tarea concreta siguiente>
- [ ] <bloqueo o decisión pendiente del usuario, si aplica>
- [x] <lo completado en este turno, como resumen breve>
```

- Sin esta sección, la respuesta se considera incompleta.
- Los pendientes deben ser accionables (qué y sobre qué archivo), no vagos ("mejorar", "revisar").

## Subagentes Disponibles

| Agente | Cuándo usar |
|--------|-------------|
| `cavecrew-investigator` | Localizar código, buscar definiciones, mapear dependencias. **Read-only.** |
| `cavecrew-builder` | Edición quirúrgica de 1-2 archivos. Refactor local, fix tipográfico, cambio de interfaz pequeño. |
| `cavecrew-reviewer` | Revisar diff, PR, o archivo completo. 1 línea por hallazgo con severidad. |

## Reglas de Derivación

1. **Scope 1 archivo, cambio obvio** → `cavecrew-builder`.  
   Ej: corregir tipo en `Ticket.ts`, renombrar prop en `TicketRow.tsx`.

2. **Scope solo búsqueda** (encontrar definiciones, usos, dependencias) → `cavecrew-investigator`.  
   Ej: "dónde se usa `transition()`", "lista de componentes que importan `Ticket`".

3. **Scope 3+ archivos o nuevo archivo** → **No delegar.** Trabajar inline.

4. **Revisión de cambios** (antes de commit o PR) → `cavecrew-reviewer`.  
   Ej: "revisa este diff", "audita `src/services/ticketService.ts`".

5. **Duda sobre qué subagente usar** → `cavecrew-investigator` primero para mapear, luego decidir.

## Convenciones de Prompt

### investigator
```
Busca [qué] en [path o patrón]. Devuelve: archivo:línea + snippet.
No sugieras fixes.
```

### builder
```
Archivo: [path]
Problema: [1-2 líneas]
Fix: [código exacto o diff]
Verificar con: [comando si aplica]
```

### reviewer
```
Revisa [archivo/diff/PR].
Formato: path:línea: <emoji> <severidad>: <problema>. <fix>.
Sin elogios. Sin scope creep.
```

## Estructura del Proyecto (referencia rápida)

```
web/src/
├── models/        → Ticket.ts, transitions.ts
├── services/      → ticketService.ts, notificationService.ts, authService.ts, storageService.ts (utilidad localStorage/URL, sin uso actual en el código)
├── hooks/         → useTicketState.ts
├── styles/        → theme.css (design system: tokens, fuentes, componentes .btn/.card/.badge/.pill, partículas)
├── utils/         → date.ts (formatRelative, formatDateTime)
├── components/    → Layout, Header, Sidebar, TicketTable, TicketRow, TicketFilters, TicketStatusBadge, NotifyButton
├── pages/         → TicketListPage, TicketDetailPage, TicketCreatePage, TicketEditPage, LoginPage, PhoneLinkPage, AssignPhonePage, WhatsAppPage
api-go/
├── main.go        → rutas HTTP: /api/tickets*, /api/notify, /api/whatsapp/{status,qr,pair,logout}, webhook del bot
├── internal/store/    → store.go (CRUD + transiciones sobre SQLite), model.go (Ticket, Event)
├── internal/evolution/ → client.go (cliente HTTP hacia Evolution API: Connect, PairingCode, Logout, SendMessage)
```

> `TicketForm`, `TicketDetail`, `TicketHistory` y `Pagination` (mencionados en versiones previas de este archivo) no existen como componentes separados: los formularios y el detalle están inline en cada page, y no hay paginación real (el backend devuelve todo el listado filtrado).

## Diseño / UI
- Design system del proyecto (paleta, tipografía, partículas CSS, componentes de UI) documentado en la skill `.claude/skills/todomark-design/SKILL.md`. Consultarla antes de tocar cualquier vista o componente visual.
- Fuente de verdad de los tokens: `web/src/styles/theme.css` (variables CSS) — no hardcodear colores/spacing nuevos fuera de ahí.

## Glosario de Archivos Clave

| Archivo | Propósito |
|---------|-----------|
| `web/src/models/Ticket.ts` | Interfaces `Ticket`, `TicketEvent`, tipos `TicketStatus`, `Priority` |
| `web/src/models/transitions.ts` | Matriz de transiciones válidas entre estados |
| `web/src/services/ticketService.ts` | Cliente HTTP hacia `api-go` (CRUD, `transition`, `setRecipient`) |
| `web/src/services/authService.ts` | Login hardcodeado `admin`/`user`, token en `localStorage` |
| `web/src/services/storageService.ts` | Utilidad genérica localStorage/URL, sin uso en el flujo de tickets actual |
| `web/src/hooks/useTicketState.ts` | Filtros en `URLSearchParams` + fetch a `ticketService.getAll` |
| `web/src/styles/theme.css` | Tokens de diseño (colores, radios, sombra), clases utilitarias y `@keyframes floatParticle` |
| `web/src/components/TicketStatusBadge.tsx` | `STATUS_META`/`PRIORITY_META` + componentes `TicketStatusBadge`, `TicketPriorityTag` |
| `web/src/utils/date.ts` | `formatRelative`, `formatDateTime` |
| `api-go/main.go` | Todas las rutas HTTP del gateway (tickets, notify, whatsapp) |
| `api-go/internal/evolution/client.go` | Cliente HTTP hacia Evolution API (QR, pairing, logout, envío) |
| `api-go/internal/store/store.go` | Persistencia SQLite de tickets + validación de transiciones |
| `docker-compose.yml` | 4 servicios: `todamark-web`(:8080), `todomark-api`(:8081), `evolution`(:3100), `postgres` (interno) |

## Flujo Típico

1. Leer `PLAN.md` y ubicar la tarea actual (§1).
2. `investigator` → localizar código afectado.
3. Decidir: `builder` (cambio pequeño) o inline (cambio grande).
4. Verificación: tsc/build del servicio (§4) + `reviewer` sobre el diff antes de commit.
5. Validar en Docker: `docker compose up -d --build` + revisar logs (§5).