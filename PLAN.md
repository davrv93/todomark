# Plan de Trabajo — TodoMark: Gestión de Tickets de Incidencia

## 1. Resumen

Aplicación web React para gestión de tickets de incidencia.  
Filtros reflejados en URL (react-router-dom v6, `useSearchParams`).  
Persistencia real en **SQLite** vía `api-go` (no localStorage: cada carga/filtro hace `fetch` al backend; `localStorage` solo se usa para el token de `authService`).  
Backend: servidor **Go** + base de datos **SQLite** (actualizado 2026-09-21 a petición del usuario; antes Node) + Evolution API para envío de notificaciones vía WhatsApp.  
Contenedor Docker (app + API gateway).  
Puertos alternos: `8080` (web), `8081` (API), `3100` (Evolution API).

---

## 2. Arquitectura General

```
┌───────────────────────────────────────────────────────────┐
│                     docker-compose.yml                      │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │  todamark-web │  │ todomark-api │  │    evolution     │   │
│  │  React (Vite) │→ │  Go (net/http│→ │ evoapicloud/     │   │
│  │  nginx :8080  │  │  + SQLite)   │  │ evolution-api    │   │
│  │               │  │  :8081       │  │  :3100           │   │
│  └──────────────┘  └──────┬───────┘  └────────┬─────────┘   │
│                            │                    │             │
│                            │            ┌───────┴──────┐      │
│                            │            │   postgres   │      │
│                            │            │  (interno)   │      │
│                            │            └──────────────┘      │
│                            └── webhook entrante (bot) ────────┘│
└───────────────────────────────────────────────────────────┘
```

- **Frontend:** React + Vite + React Router v6, `fetch` directo a `api-go` (no Axios en `ticketService`; `notificationService` sí usa Axios).
- **API Gateway:** Go (net/http) con persistencia en SQLite (`api-go/`, driver `modernc.org/sqlite` sin CGO).
- **Evolution API:** imagen oficial `evoapicloud/evolution-api`, con Postgres como base y caché local (sin Redis).
- **Persistencia:** SQLite (tickets, vía `api-go`) + URL search params (filtros de UI). `localStorage` solo guarda el token de sesión (`authService`).
- **Docker:** 4 servicios en `docker-compose.yml`: `todamark-web`, `todomark-api`, `evolution`, `postgres`.

---

## 3. Rutas y Estados en URL

| Ruta | Estado | Search Params |
|------|--------|---------------|
| `/` | Listado general | `?status=open&page=1` |
| `/ticket/:id` | Detalle | `?tab=history` |
| `/ticket/new` | Creación | `?template=default` |
| `/ticket/:id/edit` | Edición | `?field=description` |

Al cargar, el estado se lee de `URLSearchParams`.  
Al cambiar estado, se actualiza URL con `useSearchParams()` y se vuelve a pedir el listado a `api-go` (no hay caché en localStorage).

---

## 4. Modelo de Datos

```typescript
// src/models/Ticket.ts
interface Ticket {
  id: string;
  title: string;
  description: string;
  status: TicketStatus;
  priority: Priority;
  requester: string;
  assignedTo: string | null;
  createdAt: string;   // ISO
  updatedAt: string;
  history: TicketEvent[];
  whatsappChatId: string | null; // para notificar vía Evolution
}

type TicketStatus = 'open' | 'in_progress' | 'resolved' | 'closed' | 'reopened';

type Priority = 'low' | 'medium' | 'high' | 'critical';

interface TicketEvent {
  timestamp: string;
  user: string;
  action: string;
  from: string;
  to: string;
}
```

---

## 5. Árbol de Componentes

```
App (RequireAuth envuelve todo excepto /login)
├── Layout
│   ├── Header (logo TodoMark, buscar/notificaciones decorativos, avatar, partículas CSS)
│   └── Sidebar (nav: Tickets, Crear ticket, WhatsApp, Vincular teléfono, Sesión, Logout)
├── Routes
│   ├── TicketListPage → TicketFilters (pills de estado + prioridad + búsqueda) + TicketTable → TicketRow
│   │                    (sin componente de paginación: el backend devuelve el listado filtrado completo)
│   ├── TicketDetailPage (inline: cabecera, badges vía TicketStatusBadge, botones de transición
│   │                    según `TRANSITIONS[status]`, card de detalles, historial, NotifyButton)
│   ├── TicketCreatePage (form inline: título/descripción/prioridad/solicitante/WhatsApp opcional)
│   ├── TicketEditPage (form inline, mismos campos + asignado + WhatsApp)
│   ├── LoginPage (usuario/contraseña)
│   ├── PhoneLinkPage (ticketId + número + mensaje → envío directo)
│   ├── AssignPhonePage (número → `ticketService.setRecipient`)
│   └── WhatsAppPage (estado de instancia, QR, pairing code, desvincular)
```

> `TicketForm`, `TicketDetail`, `TicketHistory`, `TicketInfo`, `TicketActions` y `Pagination` no existen como componentes independientes — quedaron descartados al implementar: cada page arma su propio formulario/detalle inline reusando las clases del design system (`web/src/styles/theme.css`, ver skill `todomark-design`).

---

## 6. Servicios y Métodos

### 6.1 Servicio de Tickets (`src/services/ticketService.ts`)

| Método | Descripción |
|--------|-------------|
| `getAll(filters)` | `GET /api/tickets?status=&priority=&query=` en `api-go` |
| `getById(id)` | `GET /api/tickets/:id` |
| `create(data)` | `POST /api/tickets` (título/descripción/prioridad/solicitante/assignedTo; **no** acepta `whatsappChatId`, ver `setRecipient`) |
| `update(id, data)` | `PUT /api/tickets/:id` (el backend ignora `status` y `whatsappChatId` en este endpoint a propósito) |
| `delete(id)` | `DELETE /api/tickets/:id` |
| `transition(id, newStatus, user)` | `POST /api/tickets/:id/transition`, valida contra `TRANSITIONS` en el store Go |
| `setRecipient(id, phone)` | `POST /api/tickets/:id/recipient` — único camino para fijar `whatsappChatId` (create y edit lo llaman aparte) |
| `filter(filters)` | Alias de `getAll(filters)`, por compatibilidad con llamadas previas |

### 6.2 Servicio de Notificaciones (`src/services/notificationService.ts`)

| Método | Descripción |
|--------|-------------|
| `sendWhatsApp(ticket, event)` | POST a API Gateway → Evolution API |
| `sendStatusChange(ticket, user)` | Notifica cambio de estado |
| `sendAssignment(ticket, user)` | Notifica asignación |

### 6.3 Utilidad de almacenamiento (`src/services/storageService.ts`)

Helpers genéricos de `localStorage`/URL. **Sin uso actual**: ni `useTicketState` ni `authService` la importan (`authService` llama a `localStorage` directo). Se deja disponible por si una vista futura necesita cachear algo cliente-side; no asumir que hoy persiste nada del flujo de tickets.

| Método | Descripción |
|--------|-------------|
| `saveState(key, data)` | Guarda en localStorage |
| `loadState(key)` | Recupera desde localStorage |
| `clearState(key)` | Elimina clave |
| `syncUrlWithState(params)` | Reescribe la URL actual con `params` (no dispara render) |

### 6.4 Servicio de Autenticación (`src/services/authService.ts`)

| Método | Descripción |
|--------|-------------|
| `login(username,password)` | Verifica credenciales `admin`/`user`, guarda token en `localStorage` |
| `logout()` | Elimina token |
| `isAuthenticated()` | Devuelve `true` si token presente |

### 6.5 Hook personalizado: `useTicketState()`

```typescript
function useTicketState() {
  // Combina:
  //   - URLSearchParams (filtros: status/priority/query)
  //   - fetch a ticketService.getAll(filters) en cada cambio de filtros
  //   - useState (tickets, loading, error)
  // Retorna { tickets, filters, loading, error, dispatch: { setFilters, refresh } }
}
```

---

## 7. Máquina de Estados (Transiciones)

```
open ───→ in_progress ───→ resolved ───→ closed
  ↑                          │
  └──────── reopen ←─────────┘
```

`src/models/transitions.ts`:

```typescript
const TRANSITIONS: Record<TicketStatus, TicketStatus[]> = {
  open: ['in_progress', 'closed'],
  in_progress: ['resolved', 'open'],
  resolved: ['closed', 'reopened'],
  closed: ['reopened'],
  reopened: ['in_progress', 'closed'],
};
```

---

## 8. Integración Evolution API

Evolution API corre en contenedor separado (`:3100`).  
API Gateway (`:8081`) expone:

```
POST /api/notify
Body: { ticketId, event, chatId, message }
→ Gateway transforma y envía a Evolution POST /message/send
```

Gateway maneja autenticación, rate limiting y formateo de mensajes.

### 8.1 Auditoría del estado actual (actualizada 2026-09-21, post migración a Go)

La tabla original (Node/stub) quedó obsoleta al reescribir el backend en Go; estado real hoy:

| Componente | Estado | Nota |
|------------|--------|------|
| `NotifyButton` (`web/src/components/NotifyButton.tsx`) | ✅ | Usuario sigue fijo en `"Usuario"` (no hay usuario real logueado más allá del token); estados enviando/ok/error visibles en UI, sin `alert()` |
| `notificationService.sendWhatsApp` | ✅ | No traga errores: usa Axios sin `catch`, el error sube a quien llama |
| `api-go` `POST /api/notify` | ✅ | `502` si Evolution falla, nunca responde `sent` sin enviar |
| `api-go/internal/evolution/client.go` | ✅ | Apunta a `POST /message/sendText/{instance}` con instancia y apikey reales |
| `evolution` (imagen `evoapicloud/evolution-api`) | ✅ | Contenedor real, ya no es stub; envía WhatsApp de verdad |
| `PhoneLinkPage` | ✅ | Usa el `phone` capturado como `chatId` real al enviar |
| `AssignPhonePage` | ✅ | Nuevo: asigna `whatsappChatId` a un ticket vía `setRecipient` |
| `TicketCreatePage` / `TicketEditPage` | ✅ | Campo "WhatsApp del solicitante" opcional/editable, llama a `setRecipient` tras crear/guardar |
| `WhatsAppPage` | ✅ | Estado de instancia, QR, pairing code y botón **Desvincular** (`POST /api/whatsapp/logout`) |
| `Ticket.whatsappChatId` | ✅ | Se puede fijar desde Create, Edit, Detalle (asignar número) o PhoneLink |
| Escaneo real del QR / envío confirmado a un teléfono real | ⬜ | Sigue pendiente de verificación humana (nadie ha escaneado el QR todavía) |

### 8.2 Flujo real requerido (Evolution API)

```
1. POST /instance/create                { instanceName, qrcode: true }
2. GET  /instance/connect/{instance}    → devuelve QR para vincular el WhatsApp
3. POST /message/send/{instance}        headers: { apikey }
        body: { number: "34600123456", text: "mensaje" }
```

- `number` = destinatario con código de país, sin `+` ni espacios.
- El destinatario vive en `Ticket.whatsappChatId`, persiste en SQLite (columna `whatsapp_chat_id`), sobrevive F5 porque viene del backend, no de localStorage.
- El Gateway expone `GET /api/whatsapp/status`, `POST /api/whatsapp/qr`, `POST /api/whatsapp/pair` (código alternativo sin escanear) y `POST /api/whatsapp/logout` (desvincular sin borrar la instancia).

---

## 9. Dockerización

### `docker-compose.yml` (real, 4 servicios — ver el archivo en la raíz para el detalle exacto)

```yaml
services:
  todamark-web:      # build ./web, ports 8080:80, VITE_API_URL como build arg
  todomark-api:       # build ./api-go, ports 8081:3000, EVOLUTION_URL=http://evolution:8080
  evolution:           # image evoapicloud/evolution-api:latest, ports 3100:8080, Postgres como DB
  postgres:            # postgres:16-alpine, sin puerto expuesto al host
```

### Dockerfiles

- **web/Dockerfile:** `node:20-alpine` → build Vite → servir con `nginx:alpine` en puerto 80.
- **api-go/Dockerfile:** build Go (`CGO_ENABLED=0`, `modernc.org/sqlite`) → binario `/bin/api` en imagen mínima.
- **evolution:** imagen oficial, sin Dockerfile propio.

---

## 10. Estructura de Directorios

```
... (mantener estructura original) ...
```

---

## 11. Fases de Implementación

> Auditoría 2026-09-21: fases 1–8 implementadas (Evolution aún en modo stub).
> Las fases 9–12 habilitan el **envío real de WhatsApp con un botón a un destinatario**.

| Fase | Estado | Entregable |
|------|--------|------------|
| 1. Scaffold + Docker | ✅ | `docker compose up` levanta web, api, evolution |
| 2. Modelos + Transiciones | ✅ | `Ticket.ts`, `transitions.ts` |
| 3. Servicios + Persistencia | ✅ | CRUD, URL sync (persistencia migró de localStorage a SQLite en Fase 9) |
| 4. Componentes base + Layout | ✅ | Layout, Header, Sidebar, Routing |
| 5. Páginas de tickets | ✅ | Listado, Detalle, Crear, Editar con filtros |
| 6. Máquina de estados UI | ✅ | Transiciones visibles + validación |
| 7. API Gateway | ✅ | `/api/tickets` + `/api/notify` |
| 8. Login & rutas protegidas | ✅ | `authService`, `LoginPage`, `RequireAuth` |
| 9. Evolution API real | ✅ | Imagen `evoapicloud/evolution-api` + Postgres; instancia `todomark` creada, QR vía `POST /api/whatsapp/qr`; falta escanear el QR |
| 10. Destinatario del ticket | ✅ | `setRecipient` + campo WhatsApp en Create/Edit/AssignPhone/PhoneLink |
| 11. Botón WhatsApp end-to-end | ✅ | `NotifyButton` → Gateway → Evolution con feedback real (sin `alert`, errores visibles) |
| 12. QA + despliegue | 🔶 parcial | Build limpio (`tsc`+`vite build`+`go build`) y `docker compose up -d --build` verificados en cada tarea; **falta** el envío real confirmado (escanear QR con un WhatsApp físico) |
| 13. Rediseño UI + design system | ✅ | `theme.css`, `TicketStatusBadge`, partículas CSS, responsive; detalle en Fase 13 |
| 14. Gestión de sesión WhatsApp | ✅ | Pairing code + botón Desvincular (`POST /api/whatsapp/logout`) en `WhatsAppPage` |

**Pendiente real: solo criterio de aceptación humano de la fase 12 (escanear QR / confirmar recepción de un mensaje).**

#### Fase 9 — Evolution API real (implementada 2026-09-21)
- Imagen oficial `evoapicloud/evolution-api:latest` (v2.3.7) en `docker-compose.yml`, con Postgres 16 y caché local (sin Redis).
- Backend reescrito en **Go + SQLite** (`api-go/`): CRUD, transiciones, destinatario, notify, status y QR.
- ⚠️ Dato clave: dentro de la red Docker Evolution escucha en **8080** (`EVOLUTION_URL=http://evolution:8080`); el host lo ve en 3100 por mapeo de puertos.
- Endpoints reales verificados: `POST /instance/create`, `GET /instance/connect/{instance}` (QR en `qrcode.code`/`base64`), `GET /instance/connectionState/{instance}`, `POST /message/sendText/{instance}`.
- Variables en `.env`: `EVOLUTION_API_KEY`, `EVOLUTION_INSTANCE` (por defecto `todomark`).
- Estado: instancia creada y en `connecting`; **criterio de aceptación final pendiente de escanear el QR** desde `web/#/whatsapp` con un WhatsApp real.

#### Fase 10 — Destinatario del ticket (implementada 2026-09-21)
- `ticketService.setRecipient(id, phone)`: valida formato `^\d{8,15}$` en frontend (backend valida solo dígitos), hace `POST /api/tickets/:id/recipient` + registra evento `set_recipient` en `history`.
- `TicketCreatePage`: campo "WhatsApp del solicitante (opcional)" — tras `create()`, si hay teléfono llama a `setRecipient(created.id, phone)`.
- `TicketEditPage`: campo "WhatsApp del solicitante" pre-cargado desde el ticket; solo llama a `setRecipient` si el valor cambió (evita eventos duplicados en `history`).
- `AssignPhonePage` (ya existía) y `PhoneLinkPage` siguen como caminos alternativos para fijar/enviar a un número.
- **Criterio de aceptación:** `whatsappChatId` persiste tras F5 (verificado: viene de SQLite, no de localStorage). ✅

#### Fase 11 — Botón WhatsApp end-to-end (implementada, backend Go)
- `POST /api/notify` (Go): `502` si Evolution falla — nunca responde `sent` sin haber enviado.
- `notificationService`: sin `catch` silencioso, el error sube a `NotifyButton`.
- `NotifyButton`: estados enviando/ok/error visibles; si no hay `whatsappChatId` muestra enlace a `AssignPhonePage`.
- **Criterio de aceptación:** pulsar el botón entrega el mensaje real; los fallos se ven en la UI. ✅ (pendiente solo confirmación con teléfono real, ver Fase 12)

#### Fase 12 — QA + despliegue
- `npx tsc --noEmit` + `npm run build` (web) y `go build ./...` (api-go): verificados en cada tarea de esta sesión.
- `docker compose up -d --build` tras cada cambio, logs revisados sin errores.
- **Pendiente:** prueba humana completa (crear ticket → asignar destinatario → botón → WhatsApp recibido en un teléfono real) — requiere escanear el QR primero.
- Documentar en README las variables `.env` necesarias (aún no hay `README.md` en el repo — pendiente si se quiere entregar a terceros).

#### Fase 13 — Rediseño UI + design system (implementada 2026-09-21)
- Nuevo `web/src/styles/theme.css`: tokens de color/tipografía/radios/sombra, fuentes Space Grotesk (`--font-display`) + Public Sans (`--font-body`), clases utilitarias (`.btn*`, `.card`, `.badge`, `.pill`, `.seg-btn`, `.ticket-table`, `.detail-grid`, `.two-col-grid`) y partículas CSS (`@keyframes floatParticle` + `.particle`) en Header, topbar y card de Login.
- Nuevo `web/src/components/TicketStatusBadge.tsx`: `STATUS_META`/`PRIORITY_META` centralizan los colores por estado/prioridad; `TicketStatusBadge`/`TicketPriorityTag` reemplazan texto plano en tabla y detalle.
- Nuevo `web/src/utils/date.ts`: `formatRelative`/`formatDateTime`.
- Rediseñados: `Header`, `Sidebar` (nav con iconos SVG inline + estado activo), `Layout` (Header arriba full-width, luego Sidebar+main), `TicketFilters` (pills de estado), `TicketTable`/`TicketRow`, `NotifyButton`, y todas las pages (List/Detail/Create/Edit/Login/PhoneLink/AssignPhone/WhatsApp).
- `TicketDetailPage`: los botones de transición ahora se generan desde `TRANSITIONS[ticket.status]` (antes eran 3 botones fijos que se mostraban aunque la transición no fuera válida).
- Responsive: breakpoint único `860px` en `theme.css` (sidebar colapsa a iconos, grids de detalle/formularios a 1 columna).
- Documentado en la skill `.claude/skills/todomark-design/SKILL.md` — consultar ahí antes de tocar cualquier vista.
- **Criterio de aceptación:** `tsc --noEmit` + `vite build` limpios, `docker compose up -d --build todamark-web` con logs sin errores. ✅

#### Fase 14 — Gestión de sesión WhatsApp (implementada 2026-09-21)
- `api-go/internal/evolution/client.go`: nuevo `Client.Logout()` → `DELETE /instance/logout/{instance}` (desvincula sin borrar la instancia).
- `api-go/main.go`: nueva ruta `POST /api/whatsapp/logout`.
- `WhatsAppPage.tsx`: botón "Desvincular" siempre visible junto al estado de la instancia (deshabilitado solo si `loading` o instancia ya `close`/vacía); sección de pairing code (`POST /api/whatsapp/pair`) además del QR.
- **Sin verificar contra una instancia real conectada** (compila y pasa `go build`, pero nadie confirmó el `DELETE /instance/logout` contra el Evolution API v2.3.7 en vivo).

---

## 12. Configuración Inicial

```bash
# Clonar repositorio
mkdir todomark && cd todomark

# Crear estructura
mkdir -p web/src/{models,services,hooks,components,pages} api/src/{routes,services,middleware}

# .env
cat > .env <<EOF
EVOLUTION_API_KEY=tmk_evolution_key_2024
COMPOSE_PROJECT_NAME=todomark
EOF

# Iniciar
docker compose up -d --build
```

---

## 13. Notas Técnicas

- **Persistencia F5:** los tickets viven en SQLite (`api-go`); `useTicketState` no cachea nada en localStorage, vuelve a pedir el listado al backend en cada cambio de filtros o al montar.
- **Estado en ruta:** `useSearchParams()` de React Router. Cada filtro se refleja en URL (`?status=&priority=&query=`).
- **Transiciones:** validadas contra `AllowedTransition` en `api-go/internal/store` (espejo de `web/src/models/transitions.ts`). Llamada inválida se rechaza en el backend (`409`), no solo en la UI: la UI de `TicketDetailPage` ya solo ofrece los botones válidos según `TRANSITIONS[status]`.
- **Evolution API:** `POST /message/sendText/{instance}` con header `apikey`. Gateway oculta instancia y apikey del frontend.
- **Puertos:** `8080` (web), `8081` (api gateway), `3100` (evolution, host) / `evolution:8080` (red interna Docker).

---

### 8.3 Chatbot entrante (2026-09-21)

- Evolution envía `MESSAGES_UPSERT` al API Go vía `POST /webhook/evolution` (registrado al arrancar con `WEBHOOK_URL`).
- **Solo responde a números registrados como destinatarios desde la UI** (`whatsapp_chat_id` en SQLite): a cualquier otro número ignora en silencio.
- Flujo: `menu`/`hola` → lista de tickets del remitente (si hay uno solo, menú directo) → `1` estado, `2` descripción, `3` asignado a, `4` última novedad, `5` salir. Sesión por número con TTL de 10 min.
- Notas de voz/imágenes/documentos: se aceptan y se guía de vuelta al menú (sin transcripción; requiere STT como Whisper si se desea).

## 14. Login y Vínculo de Teléfono

### 14.1 Servicio de Autenticación (`src/services/authService.ts`)
| Método | Descripción |
|--------|-------------|
| `login(username,password)` | Verifica credenciales `admin`/`user`, guarda token en `localStorage` |
| `logout()` | Elimina token |
| `isAuthenticated()` | Devuelve `true` si token presente |

### 14.2 Página de Login (`src/pages/LoginPage.tsx`)
- Formulario usuario/contraseña.
- Envío llama a `authService.login`.
- Redirección a `/` si éxito, muestra error si falla.

### 14.3 Protección de Rutas (`RequireAuth`)
- Componente que verifica `authService.isAuthenticated()`.
- Envuelve todas las rutas (excepto `/login`) en `App.tsx`.

### 14.4 Página de Vínculo de Teléfono (`src/pages/PhoneLinkPage.tsx`)
- Formulario con `ticketId`, `phoneNumber`, `message`.
- Envío ejecuta `POST /api/notify` usando `notificationService`.
- Muestra confirmación o error.

### 14.5 Integración en UI
- Añadir enlaces `/login` y `/phone-link` en `Sidebar`.
- Botón “Logout” que llama a `authService.logout()` y redirige a `/login`.

---
