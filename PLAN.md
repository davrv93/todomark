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
| 15. Rediseño Apple-minimal + motion | ✅ | 7 fases (tokens→QA), detalle en §15 |
| 16. Fase 2 — Núcleo + Canales + GLPI + Hardening | ✅ | Email/equipo/GLPI, Kanban, Reportes, RBAC, config.js runtime; resumen en §16, detalle completo en `2da fase.md` §5 |
| 17. Fase 3 — OAuth2/OIDC (Hydra) + Astro Islands | ✅ | Login SSO end-to-end verificado con curl real (login→consent→code→token→introspect); resumen en §17 |
| 18. Fase 4 — Bandeja WhatsApp + presencia en vivo | ✅ | `/messages` con paginación cursor, `/api/presence/stream` (SSE); resumen en §18 |
| 19. Fase 5 — Analítica ClickHouse | ✅ (2026-09-23) | ClickHouse desplegado, `fact_tickets`/`fact_events` sincronizando cada 5 min con TODOS los campos (incluye `channel`/`category`/`building_id`/`satisfaction_score`, agregados en Fase 7). 7 vistas SQL + **Grafana** (`:3300`) provisionado por código (datasource + dashboard "TodoMark — Ejecutivo", 10 paneles), verificado con queries reales vía API de Grafana. **Falta (no pedido):** dbt, GraphQL, `fact_notifications`/`dim_*` sin poblar (el store no modela esas entidades). Ver §19 y §21 |
| 20. Fase 6 — Landing page + chatbot Gemini/DeepSeek + CMS Directus | 🔶 parcial | Landing real en `/landing`, leads en SQLite, chatbot con Gemini Flash Lite (primario) + DeepSeek (respaldo) + clasificador + RAG (`knowledge_chunks`), keys configurables desde `/settings` sin tocar env vars. **CMS Directus real** (puerto 8083), landing consume contenido de verdad (verificado con prueba de marcador). Contador de visitas real (`POST /api/landing/visit`, Fase 7 §21) alimenta el embudo comercial. **Falta:** SEO/Lighthouse, keys reales de Gemini/DeepSeek. Ver §20 |
| 21. Fase 7 — Catálogo de reportes (33), edificios/CSAT/escalamiento demo, integración en `web/`, seed 1000, presencia y logins demo | ✅ (2026-09-23) | Ver §21 — resumen completo |
| 22. Fase 8 — Dashboard drag-and-drop, bot de WhatsApp con registro de tickets (fotos/audio/transcripción), reporte por WhatsApp (texto+PDF), cheat sheet Qwik, exposición por túnel + candado de seguridad | ✅ (2026-09-23) | Ver §22 — resumen completo |

**Pendiente real:** criterio de aceptación humano de la fase 12 (escanear QR / confirmar recepción de un mensaje real — YA CUMPLIDO en Fase 8, ver §22.1), keys reales de Gemini/DeepSeek para Fase 6, y decidir si reemplazar los datos DEMO de Fase 7 (edificios, tarifa, umbral SLA) por datos reales del negocio antes de cualquier entrega a un gerente real.

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

## 15. Plan de diseño Apple-minimal + motion (2026-09-23)

### 15.1 Análisis del estado actual
- **Semántica:** `Layout.tsx` apila `Header` (60px, full width) sobre fila `Sidebar` (210px) + `main.page-main` (padding 24/28). Tablas densas con `min-width: 720px` y scroll en `.table-wrap`. Detalle en `.detail-grid` (1.55fr/1fr). Formularios en `.form-card` (`max-width: 620px`).
- **Diseño:** UI compacta (12–14px), títulos `Space Grotesk`, cuerpo `Public Sans`, radios 10/7px, una sola sombra sutil, sin gradientes.
- **Posiciones:** header `relative` con `overflow: hidden`; partículas en capa `absolute z0`, contenido en `z1`. Sidebar fijo a la izquierda; avatar y badges inline.
- **Estilos:** tokens en `web/src/styles/theme.css` (`--bg`, `--surface`, `--border`, `--ink*`, `--primary*`, pares semánticos teal/amber/red/gray + tint). Clases reutilizables `.btn*`, `.card`, `.badge`, `.pill`, `.seg-btn`, `.ticket-table`. Fuente de verdad: `theme.css` + skill `todomark-design`.
- **Animaciones:** solo `@keyframes floatParticle` (6s, opacity .2–.5, translate 4/−8px) en Header/topbar/Login, más hover de fila (.12s). No hay transiciones de página, stagger, press states, skeletons, ni respeto a `prefers-reduced-motion`.
- **Colores:** paleta fría confiable (azul primario + slate + semánticos). Contraste de texto correcto; partículas en `--primary`. Falta aire, blur y física suave propios del minimalismo Apple.

### 15.2 Fases de trabajo
1. **Fase 1 — tokens de motion base:** añadir `--ease-out`, `--ease-spring`, `--dur-fast/med/slow`, `--blur-sm/md`; anillo `focus-visible`, `::selection`, scrollbar sutil y `prefers-reduced-motion: reduce`. Verificación: `npx tsc --noEmit` + `npm run build`.
2. **Fase 2 — header/hero con partículas vivas:** densidad según viewport, pausa offscreen (IntersectionObserver), glow blur tras la marca, hover +1px con sombra, press scale .97, pulso teal en badge online. Partículas prohibidas en tablas/listas.
3. **Fase 3 — lista con conexión:** entrada fade+rise 180ms, stagger 30ms, `.pill` con morph y spring, búsqueda expansible al foco, row press scale .99, historial como timeline con dot animado.
4. **Fase 4 — detalle/transiciones sentidas:** badge con morph de color + check SVG dibujado, micro-shake solo en error, `NotifyButton` con spinner→check sin saltos de layout.
5. **Fase 5 — formularios/login cálidos:** focus eleva 2px con borde primary, error con vibración única de 4px, éxito con check teal, login con blur y entrada escalonada.
6. **Fase 6 — mobile divergente:** sidebar 60px + bottom sheet de acciones, tabla→cards, filtros sticky con scroll-snap, detalle como sheet full-screen con drag-close, targets táctiles 44px. Solo crear UI separada si la tabla resulta ilegible en pruebas.
7. **Fase 7 — QA motion/perf/a11y:** 60fps sin layout shift, contraste AA, teclado completo, `docker compose up -d --build` con logs de `todamark-web` limpios.

### 15.3 Prompts para mantener el feeling Apple
- **Global (pegar antes de cada tarea):** «Minimalismo Apple. Aire, blur, física suave. Motion con propósito. Sin gradientes, sin emoji, sin sombras duras. Solo tokens de `theme.css`. 60fps. Respeta `prefers-reduced-motion`.»
- **Fase 1:** «Define `--ease-out`, `--ease-spring`, `--dur-fast/med/slow`, `--blur-sm/md`. Anillo `focus-visible` en `primary-tint`. Reduce-motion desactiva todo.»
- **Fase 2:** «Partículas sutiles solo en hero/login/Header. Densidad según viewport. Pausa offscreen. Opacity .2–.5. Nada dentro de tablas. Glow blur tras la marca.»
- **Fase 3:** «Entrada fade+rise 180ms, stagger 30ms. Pill con morph y spring. Row press scale .99. Hover con elevación mínima. Sin saltos de layout.»
- **Fase 4:** «Cambio de estado con morph de color + check SVG dibujado. Error con micro-shake único. `NotifyButton` spinner→check con texto estable.»
- **Fase 5:** «Focus eleva 2px, borde primary, label en primary. Error vibra 4px una sola vez. Éxito con check teal.»
- **Fase 6:** «Touch 44px. Tabla→cards. Filtros sticky con scroll-snap. Detalle como sheet full-screen con drag-close. Acciones primarias abajo.»
- **QA final:** «Revisa como diseñador Apple: ¿respira? ¿cada animación comunica algo? ¿60fps? ¿teclado completo? ¿contraste AA? Elimina todo motion decorativo sin función.»

---

## 16. Fase 2 — Núcleo + Canales + GLPI + Hardening (resumen, ✅ implementada)

> Consolidado 2026-09-23: este plan vivía suelto en `2da fase.md`. El detalle completo (tareas, criterios de aceptación, mapeo de campos GLPI, riesgos) queda ahí como registro histórico; acá solo el estado.

| Etapa | Estado | Entregable real |
|-------|--------|------------------|
| 2A — Núcleo tickets | ✅ | Migración `email`/`assigned_team`/`closed_at`, `ListByEmail`/`ListByTeam`, Kanban (`@dnd-kit`), Reportes (Chart.js), `/api/reports/summary` |
| 2B — Canales (email + WhatsApp) | ✅ | Webhook `/webhook/email` con idempotencia (`processed_events`), cliente Meta WhatsApp con retry≤3 en 429 |
| 2C — GLPI | ✅ | Cliente con sesión (`initSession`/`killSession`), sync cada 5 min, tabla `glpi_conflicts`, badge de sincronización en `TicketDetailPage` |
| 2D — Hardening | ✅ | RBAC por scope (`withAuth`/`hasRole`, admin/agent/viewer), healthchecks, logs JSON, `config.js` runtime para `VITE_API_URL` |

## 17. Fase 3 — OAuth2/OIDC (Hydra) + Astro Islands (resumen, ✅ implementada)

| Etapa | Estado | Entregable real |
|-------|--------|------------------|
| 3A — Hydra OIDC | ✅ | `oryd/hydra:v2.2.0` desplegado (migrate+serve), cliente público `todomark-web` registrado, provider de login/consent propio (`/api/hydra/login`, `/api/hydra/consent`) porque Hydra no trae UI propia. Login viejo admin/user queda de fallback si `VITE_HYDRA_*` no está configurado. **Verificado con curl real**: auth→login→consent→code→token→introspect, token con `scope:"openid offline_access admin"` |
| 3B — Astro Islands | ✅ | `astro-frontend/` (puerto 8082), Kanban (`client:load`) y Reportes (`client:visible`) como islands lazy, healthcheck propio |

## 18. Fase 4 — Bandeja de mensajes + presencia en tiempo real (resumen, ✅ implementada)

| Etapa | Estado | Entregable real |
|-------|--------|------------------|
| 4A — Bandeja | ✅ | Tabla `whatsapp_messages`, `INBOX_SHOW_ALL` configurable, `/api/messages/threads` + `/api/messages/threads/{chatId}` + `POST /api/messages`, paginación cursor (`limit`/`before`), página `/messages` con selector Número/Grupo |
| 4B — Presencia | ✅ | SSE (`GET /api/presence/stream`, sin dependencias nuevas), badge "N en línea" con pulso en `Header` |

---

## 19. Fase 5 — Analítica ClickHouse (🔶 parcial — ClickHouse + ETL real, dashboards pendientes)

> Consolidado 2026-09-23 desde `plan_trabajo_clickhouse.md` + `clickhouse_reportes.md` (quedaban sueltos, sin estado). Implementado y verificado con datos reales: ClickHouse desplegado, `fact_tickets`/`fact_events` sincronizando. Sin dbt/GraphQL/Superset/Metabase/Grafana — ver §19.10.

### 19.1 Contexto y objetivo
Proveer una capa de reporting/analítica que refleje la eficiencia del soporte (TodoMark), separada de la base operacional (SQLite vía `api-go`).

### 19.2 Arquitectura propuesta

| Capa | Tecnologías | Comentario |
|------|-------------|------------|
| Ingesta | `api-go` → CDC (sqlite-to-clickhouse) con `go-sqlite3` + `clickhouse-go` | Captura cambios en `tickets`, `events`, `users`, `clients` |
| Almacenamiento | ClickHouse (cluster replicado) | Columnar, alta ingestión, consultas analíticas rápidas |
| Transformación | Materialized Views + `dbt` | Normaliza eventos, crea hechos/dimensiones |
| Exposición | API GraphQL (Go) → Superset / Metabase | Dashboards interactivos y exportables |
| Monitorización | Prometheus + Grafana | Salud del pipeline y SLA |

### 19.3 Modelo de datos (ClickHouse)

**Hechos:** `fact_tickets` (una fila por ticket), `fact_events` (cambios de estado/prioridad/asignación), `fact_notifications` (envíos WhatsApp/Email).
**Dimensiones:** `dim_client`, `dim_user` (agentes), `dim_time` (calendario con huso/week/month/quarter).

```
-- fact_tickets (campos clave)
TicketID   UInt64
CreatedAt  DateTime
ClosedAt   DateTime Nullable
Status     Enum8('open','in_progress','resolved','closed')
Priority   Enum8('low','medium','high','critical')
ClientID   UInt64
AgentID    UInt64 Nullable
Category   LowCardinality(String)
```

### 19.4 KPIs principales

| KPI | Fórmula | Fuente | Frecuencia |
|-----|---------|--------|------------|
| Tickets creados (día) | `COUNT(*) WHERE CreatedAt >= today()` | fact_tickets | Diario |
| MTTR (tiempo medio de resolución) | `AVG(ClosedAt - CreatedAt) WHERE Status='closed'` | fact_tickets | Diario |
| Tickets por prioridad | `COUNT(*) GROUP BY Priority` | fact_tickets | Diario |
| Tickets por agente | `COUNT(*) GROUP BY AgentID` | fact_tickets | Diario |
| SLA cumplido | `COUNT(*) WHERE (ClosedAt-CreatedAt)<=SLA_LIMIT / TOTAL` | fact_tickets | Diario |
| Notificaciones enviadas | `COUNT(*)` | fact_notifications | Diario |
| Errores críticos | `COUNT(*) WHERE Priority='critical'` | fact_tickets | Diario |

### 19.5 Queries de referencia (KPIs)

```sql
-- Tickets creados por día
SELECT toDate(CreatedAt) AS day, count() AS tickets
FROM fact_tickets WHERE CreatedAt >= today() - INTERVAL 30 DAY
GROUP BY day ORDER BY day;

-- MTTR en horas
SELECT avg(toUnixTimestamp(ClosedAt) - toUnixTimestamp(CreatedAt)) / 3600 AS mttr_hours
FROM fact_tickets WHERE Status = 'closed' AND ClosedAt IS NOT NULL;

-- Tickets por prioridad
SELECT Priority, count() AS tickets FROM fact_tickets GROUP BY Priority;

-- Top 10 agentes
SELECT AgentID, count() AS tickets FROM fact_tickets
WHERE AgentID IS NOT NULL GROUP BY AgentID ORDER BY tickets DESC LIMIT 10;

-- Cumplimiento SLA (%)
SELECT round(100 * sum(if(ClosedAt - CreatedAt <= SLA_LIMIT, 1, 0)) / count(), 2) AS sla_pct
FROM fact_tickets WHERE Status = 'closed';

-- Notificaciones por día
SELECT toDate(SentAt) AS day, count() AS notifications
FROM fact_notifications WHERE SentAt >= today() - INTERVAL 30 DAY
GROUP BY day ORDER BY day;

-- Últimos 20 tickets críticos
SELECT TicketID, CreatedAt, Priority, Status FROM fact_tickets
WHERE Priority = 'critical' ORDER BY CreatedAt DESC LIMIT 20;
```

### 19.6 Queries para visualización (Grafana / Superset / Metabase)

| # | Query | Visualización |
|---|-------|----------------|
| 1 | Serie temporal tickets creados vs cerrados (`countIf(Status='open')` / `countIf(Status='closed')` por día, 60 días) | Línea dual (área) |
| 2 | Heatmap `toHour(CreatedAt)` × `toDayOfWeek(CreatedAt)` | Heatmap |
| 3 | Top 10 agentes: `count()` + `avg(resolución)` agrupado por `AgentID` | Barras + línea |
| 4 | Distribución de prioridad (`GROUP BY Priority`) | Donut / Pie |
| 5 | SLA gauge (% cumplimiento) | Gauge, objetivo 95% |
| 6 | Mapa de clientes (join `fact_tickets`+`dim_client`, lat/long) | Mapbox / Leaflet |
| 7 | Tabla extensiva de eventos críticos (`Priority='critical'`, con `AgentID`/`Category`) | Tabla con filtros |

### 19.7 Análisis recomendado
- Tendencia semanal (series de 7 días) para detectar picos de carga.
- Correlación SLA vs prioridad (join `fact_tickets`+`dim_time`).
- Productividad por agente (KPI tickets/agente + MTTR combinados).
- Eficacia de notificaciones (`fact_notifications` + `fact_events` por `TicketID`).
- Geografía: clustering (k-means) sobre lat/long para regiones con mayor incidencia.

### 19.8 Roadmap y estado

| Sprint | Actividad | Entregable | Estado |
|--------|-----------|------------|--------|
| 0 | Preparar entorno ClickHouse (docker-compose) | Cluster local + usuarios DB | ✅ |
| 1 | ETL: CDC SQLite → ClickHouse (Go worker) | `internal/clickhouse/sync.go`, sin tests unitarios (no pedidos) | 🔶 solo `fact_tickets`/`fact_events`; `fact_notifications`/`dim_*` creadas pero vacías |
| 2 | Modelado dbt + materialized views | Esquema de hechos/dimensiones | ⬜ |
| 3 | Exposición GraphQL + autenticación | `/graphql` con filtros KPI | ⬜ |
| 4 | Dashboard en Superset/Metabase | Tablero "Support Overview" | ⬜ |
| 5 | Alertas SLA (Grafana) + documentación | Alertas + run-book | ⬜ |
| — | Dashboards: Superset `Support Overview`, réplica en Metabase, panels Grafana con datasource ClickHouse + alerta SLA >95%, SQL versionado en `infra/sql/` | 4 pasos de `clickhouse_reportes.md` §4 | ⬜ |

### 19.9 Calidad y riesgos
- Consistencia: `DateTime64(3)` (milisegundos).
- Seguridad: roles `read_only`/`analyst`/`admin` en ClickHouse.
- Performance: `PRIMARY KEY (TicketID, CreatedAt)`, `ORDER BY (CreatedAt)`.
- Retención: TTL 90 días en `fact_events`.
- Backup: snapshots diarios a S3.

### 19.10 Estado (actualizado 2026-09-23) y próximo paso
Implementado y verificado con datos reales: servicio `clickhouse` en `docker-compose.yml` (imagen oficial, healthy), `api-go/internal/clickhouse/{schema.sql,client.go,sync.go}`, sync cada 5 min contra `fact_tickets`/`fact_events` (conteo confirmado subiendo de 4 a 5 tras crear un ticket de prueba). `dim_client`/`dim_user`/`dim_time` existen pero no se pueblan (el store no modela esas entidades con ID numérico, son texto libre). `fact_notifications` vacía (no hay tabla de notificaciones enviadas en el store todavía).

**Próximo paso real:** decidir si vale la pena modelar `dim_client`/`dim_user` en el store (hoy `Requester`/`AssignedTo` son texto libre, no IDs) antes de seguir con dbt — sin eso, las dimensiones de ClickHouse quedan huecas y dbt no tiene mucho que modelar todavía.

---

## 20. Fase 6 — Landing page + chatbot Gemini/DeepSeek + CMS Directus (🔶 parcial — landing, leads, CMS y chatbot RAG reales; SEO y keys de producción pendientes)

> Consolidado 2026-09-23 desde `plan_trabajo_website.md` (quedaba suelto, sin estado). Nada de esto existe: no hay proyecto de landing, no hay CMS, no hay integración DeepSeek. Nota: esto es un producto de marketing (landing pública) separado de la app operacional (`web/`) — decisión pendiente de si vive en un servicio nuevo en `docker-compose.yml` o en `astro-frontend/` reutilizando esa base Astro ya desplegada.

### 20.1 Visión
Landing "single page" para el ERP TodoMark (framework JSR): vende beneficios, muestra casos de uso, captura leads. Chatbot conversacional con **DeepSeek** para guiar usuarios. CMS admin para publicar contenido sin tocar código.

### 20.2 Requisitos

| Tipo | Detalle |
|------|---------|
| Funcional | Presentación de beneficios · formulario de lead (nombre/email/empresa) · botón "Solicitar demo" → webhook · chatbot integrado (modal) · CMS admin WYSIWYG (texto/imágenes/bloques) |
| No funcional | SEO · responsive · WCAG 2.1 AA · carga < 2s · navegadores modernos · seguridad (CSP, XSS) |

### 20.3 Arquitectura propuesta
- **Frontend:** Vite + React (TS), UI Apple-like, tokens de `theme.css` (mismo design system que `web/`).
- **Chatbot:** cliente JS → API DeepSeek (`https://api.deepseek.com/v1/chat/completions`, REST). Contexto: URL actual + snippets del CMS.
- **CMS:** headless (Strapi o Directus), API GraphQL/REST, admin en `/admin`.
- **Backend:** Node/Express o Go-gateway para formularios/webhook a CRM.
- **Hosting:** Docker Compose — nginx front, API, CMS (+ ClickHouse para analytics, ver Fase 5).

### 20.4 Secciones de la landing
1. Hero (título, subtítulo, CTA "Solicitar demo")
2. Beneficios (4-5 ítems, iconos SVG — integración total con JSR, escalabilidad, UX Apple-like, automatización, soporte 24/7)
3. Casos de éxito (carousel testimonios/logos)
4. Características (tabla comparativa JSR vs competidores)
5. Demo video (embed)
6. Formulario de lead → CRM
7. Chatbot (botón flotante + modal IA)
8. Footer (legales, redes, contacto)

### 20.5 Chatbot — implementado (2026-09-23): Gemini Flash Lite + clasificador + RAG
- Prompt base original (DeepSeek, respaldo): *"Actúa como asistente de ventas de Todo Mark ERP. Responde en español, tono profesional, sugiere demo o contacto. Respuestas ≤ 2 frases."*
- **Motor principal: Gemini Flash Lite** (`api-go/internal/gemini/client.go`, mismo patrón `NewFromEnv`/`SetAPIKey`/`Configured` que `deepseek`). Si no está configurado, cae a DeepSeek; si ninguno lo está, fallback textual a formulario de contacto.
- **Clasificador real** (`internal/gemini/classify.go`): cada mensaje se clasifica en `demo`/`soporte`/`queja`/`otro` vía prompt al LLM (no es un modelo de ML entrenado aparte — clasificación honesta por prompt).
- **RAG real** (`internal/store` tabla `knowledge_chunks`): antes de responder, `SearchKnowledge(mensaje, 3)` recupera los chunks más relevantes (por palabras clave, sin vector store) y se inyectan como contexto en el prompt. Cada intercambio del chat se guarda como chunk nuevo — así la base crece con el uso real (esto es lo que hace que "aprenda": más contexto disponible con el tiempo, no reentrenamiento del modelo).
- `POST /api/chat` devuelve `{"reply": "...", "category": "demo"}` (category solo si Gemini clasificó).
- **Configurable desde la UI**: `web/src/pages/SettingsPage.tsx` (ruta `/settings`, dentro de la app interna, detrás de login) — cargar/quitar las keys de Gemini y DeepSeek sin tocar `.env` ni reiniciar el contenedor (`GET/POST/DELETE /api/settings/{gemini,deepseek}`, guardadas en tabla `settings` de SQLite, aplicadas en caliente vía `SetAPIKey`).
- UX: botón redondo "?" esquina inferior derecha → modal input+respuesta (ya implementado en `landing.astro`).

### 20.6 CMS Admin — implementado (2026-09-23): Directus, no Strapi
- **Decisión tomada:** Directus (el usuario la resolvió explícitamente), no Strapi.
- Desplegado en `docker-compose.yml`, servicio `directus` (imagen oficial, puerto **8083**→8055 interno), DB propia `directus` en la MISMA instancia de Postgres que ya usa Hydra (mismo patrón, sin Postgres nuevo).
- Colección `landing_content` (`key`/`value`) con lectura pública, sembrada con el hero + 5 beneficios (mismo texto que estaba hardcodeado). Admin Directus: `http://localhost:8083` (credenciales dev abajo).
- `astro-frontend/src/pages/landing.astro` consume `landing_content` en **build time** (no runtime: es un sitio estático) vía `PUBLIC_DIRECTUS_URL`, con fallback al texto hardcodeado si Directus no responde. **Verificado de verdad**: se cambió `hero_title` en Directus a un marcador, se reconstruyó, y el HTML servido por `/landing` mostró el marcador (descarta coincidencia con el fallback).
- **Caveat operativo:** por ser build-time, editar contenido en Directus NO se refleja con un `docker compose up -d --build astro-frontend` normal (Docker cachea la capa de build) — hace falta `docker compose build --no-cache astro-frontend && docker compose up -d astro-frontend`. No hay cache-busting automático todavía (deuda anotada, no improvisada).
- Credenciales dev (**rotar antes de cualquier despliegue real**): admin `admin@todomark.dev` / `todomark_admin_2026` (override `DIRECTUS_ADMIN_EMAIL`/`DIRECTUS_ADMIN_PASSWORD`); `KEY`/`SECRET` con defaults `_change_me` (override `DIRECTUS_KEY`/`DIRECTUS_SECRET`).
- Editor drag-and-drop rico (bloques `hero`/`benefit`/`testimonial`/`video`/`form`/`customHTML` del diseño original) y auth JWT+RBAC propia — **no implementado**: Directus ya trae su propio admin UI y su propio sistema de permisos (policies), no hace falta reinventar `withAuth`/`hasRole` de `api-go` para esto.

### 20.7 Roadmap y estado

| Sprint | Actividad | Entregable | Estado |
|--------|-----------|------------|--------|
| 0 | Setup proyecto (repo, Docker, CI) | Reutilizó `astro-frontend/` ya desplegado (decisión tomada: sin servicio nuevo) | ✅ |
| 1 | Landing static UI + responsive | `astro-frontend/src/pages/landing.astro`, contenido real + placeholders explícitos donde falta dato | ✅ |
| 2 | Integrar CMS headless (**Directus**, decidido) | Directus corriendo (:8083), colección `landing_content` | ✅ |
| 3 | Conectar UI con CMS | Landing consume contenido dinámico (verificado con prueba de marcador) | 🔶 vía REST en build-time, no GraphQL runtime — ver caveat de cache en §20.6 |
| 4 | Chatbot Gemini (primario) + DeepSeek (respaldo) + clasificador + RAG | `POST /api/chat`, keys configurables desde `/settings` | 🔶 wireado, probado, sin keys reales todavía |
| 5 | Formulario lead → webhook CRM | `POST /api/leads` → tabla `leads` en SQLite | 🔶 persiste real, sin CRM externo (no hay uno configurado) |
| 6 | SEO + performance (Lighthouse > 90) | Audits, lazy-load, optimización imágenes | ⬜ |
| 7 | QA, accesibilidad, documentación | Test de usabilidad, manual admin | ⬜ (accesibilidad básica sí: `<button>`/`<label>` reales, no auditado con Lighthouse) |

### 20.8 Métricas de éxito
- Conversión leads/visitas ≥ 5%.
- Carga < 2s (FCP).
- Chatbot: sesiones ≥ 30s, satisfacción ≥ 4/5.
- Publicación CMS < 5 min.

### 20.9 Riesgos
- Dependencia de IA → fallback a FAQ estática.
- Seguridad CMS → hardening, CSP, rate-limit.
- SEO → prerendering (React-Snap) o SSR opcional.
- Escalabilidad → CDN + caching de consultas CMS.

### 20.10 Estado (actualizado 2026-09-23) y próximo paso
Arquitectura decidida y construida: landing en `astro-frontend/src/pages/landing.astro` (ruta `/landing`, puerto 8082), sin servicio nuevo. Leads reales en SQLite (`POST /api/leads`), chatbot wireado contra `internal/deepseek` con fallback honesto si no hay `DEEPSEEK_API_KEY` (confirmado: sin key, no sale ninguna llamada HTTP real a DeepSeek).

**Bug encontrado y arreglado durante la verificación:** nginx de `astro-frontend` redirigía `/landing` → `http://localhost/landing/` (sin el puerto 8082, por el mapeo de puertos de Docker) — cualquier link sin `/` final rompía. Arreglado con `absolute_redirect off;` en `nginx.conf`.

**Próximo paso real:** decidir CMS (Strapi vs Directus) — sigue siendo la decisión que bloquea Sprint 2+; hasta entonces el contenido de la landing es hardcodeado en el `.astro`, lo cual es válido para Sprint 0-1.

---

## 21. Fase 7 — Catálogo de reportes, edificios demo, ClickHouse+Grafana, seed y demo tooling (2026-09-23, ✅ implementada)

> Detalle completo en `DOCUMENTACION.md` (§7-§15 catálogo de reportes, §11 estado por fase) — este bloque es el resumen ejecutivo para `PLAN.md`.

### 21.1 Qué se construyó
- **Catálogo de 33 reportes** en 4 capas (Operativos O1-O8, Gerenciales G1-G11, Usuario final U1-U6, Analíticos A1-A10). 32 implementados; A1-A10 (ML/inferencia) deliberadamente NO iniciados — requieren >500 tickets según el propio diseño, y aunque ahora hay >1000 (tras el seed), no se pidieron explícitamente y ameritan su propia tarea.
- **Backend** (`api-go`): `ExecutiveSummary`/`ReportSummary` extendidos con `byAgent`/`byChannel`/`byCategory`/`byBuilding`/`categoryByBuilding`/`agingHoursByStatus`/`unassignedCount`/`reopensToday`/`slaAtRiskCount`/`slaOverdueCount`/`frtHours`/`weeklyBacklog`/`weeklyTrend`/`csatAvg`/`slaCompliancePct`/`costEstimateDemo`/`landingVisits`/`leadsTotal`. Nuevos campos en `tickets`: `channel`, `category`, `building_id`, `unit_id`, `satisfaction_score` (todos nullable, mismo patrón `ensureColumn` que ya existía para `email`/`closed_at`). Nueva tabla `buildings` (3 filas **DEMO**, ver §21.3) y `landing_visits` (contador real). Nuevos endpoints: `GET /api/reports/executive`, `GET /api/buildings`, `POST /api/tickets/{id}/satisfaction`, `POST /api/tickets/{id}/escalate`, `POST /api/landing/visit`.
- **Frontend interno (`web/`, :8080):** el dashboard gerencial (`/executive`) y operativo (`/reports`) quedaron integrados en la SPA existente (`Sidebar`/`Layout`/login, F5 persiste vía `try_files` de nginx) — ya no viven solo en `astro-frontend` sin sidebar.
- **Frontend público (`astro-frontend`, :8082):** `/mis-tickets` (portal cliente, U1/U5/U6 con estrellas de calificación y botón "Escalar a gerente") y `/ejecutivo` (versión original, se mantiene además de la de `web/`).
- **ClickHouse + Grafana (F5 completa):** `fact_tickets` sincroniza los campos nuevos, 7 vistas SQL, servicio `grafana` nuevo en `docker-compose.yml` (`:3300`, datasource+dashboard provisionados por código, sin clicks manuales). Verificado con queries reales vía `/api/ds/query` de Grafana, no solo logs.
- **Seed de demo:** `api-go/cmd/seed/main.go` (programa standalone, NO se compila en la imagen de producción) generó 1000 tickets repartidos en 270 días, con status/prioridad/canal/categoría/edificio/CSAT/reaperturas realistas y con variancia de SLA (no todo 100% ni 0% cumplido). Total real: 1009 tickets.
- **Presencia en vivo en superficies públicas:** badge "N en línea" (ya existía en `web/`, Fase 4B) portado a `astro-frontend` (`ejecutivo`, `mis-tickets`, `landing`) reusando el mismo `GET /api/presence/stream`, sin tocar backend.
- **4 logins demo** en `web/` para mostrar la app: `admin`/`user` (el original, sin cambios), `agente`/`agente123`, `gerente`/`gerente123`, `viewer`/`viewer123`. Sin RBAC real detrás — son personas de demo, no un control de acceso (documentado explícitamente para no confundirlo con seguridad real).

### 21.2 Ejecución con subagentes en paralelo
Presencia-en-`astro-frontend` y logins-demo-en-`web/` se delegaron a dos agentes en paralelo (archivos no superpuestos), mientras el seed y esta actualización del plan se hicieron inline — siguiendo la regla de derivación de `AGENTS.md` (scope acotado a 2-4 archivos por agente, verificación tsc/build/Docker exigida a cada uno).

### 21.3 Honestidad de datos — qué es real y qué es DEMO
Regla aplicada de forma consistente en toda la Fase 7 (ver `DOCUMENTACION.md` §13/§15): nada se inventa silenciosamente, todo lo ficticio queda etiquetado.

| Dato | Real o demo | Nota |
|---|---|---|
| Canal, categoría, CSAT, escalamiento, aging, FRT, backlog semanal | **Real** | Se computan de timestamps/eventos reales; los tickets viejos sin el campo quedan como `sin_canal`/`sin_categoria`, no se rellenan retroactivo |
| Umbral SLA (crítico 4h/alta 24h/media 72h/baja 168h) | **Demo** | Constante Go, no confirmada por un gerente real — etiquetado "(demo)" en toda la UI |
| 3 edificios (Torre Norte, Edificio Sur, Residencial Central) | **Demo** | Sembrados en `migrate()`, nombres explícitamente marcados "(demo)" |
| Tarifa $25/hora (costo estimado, G8) | **Demo** | Constante Go `costoHoraDemo` |
| Visitas a la landing (G11) | **Real** | Contador iniciado 2026-09-23, sin histórico previo inventado |
| Los 1000 tickets del seed | **Demo, explícito** | Generados por `cmd/seed`, no son incidencias reales de ningún cliente |
| G9 (proveedores/contratistas) | **No implementado a propósito** | No es un dato faltante sino un concepto que el modelo de `Ticket` no tiene — forzarlo sería inventar una entidad de negocio nueva |

### 21.4 Próximo paso real
Si este proyecto deja de ser demostrativo: reemplazar umbral SLA, tarifa y edificios por datos reales del negocio (F0 del catálogo de reportes, reunión con el gerente real) antes de cualquier entrega. Hasta entonces, decidir si conservar los duplicados de Ejecutivo/Reportes en `astro-frontend` ahora que viven en `web/`, y si se necesita RBAC real detrás de los 4 logins demo.

---

## 22. Fase 8 — Dashboard personalizable, bot de WhatsApp con registro de incidentes, reporte por WhatsApp, cheat sheet Qwik y exposición pública (2026-09-23, ✅ implementada)

### 22.1 WhatsApp real: hito cumplido
Se escaneó el QR con un teléfono real (+51 992 621 314) y la instancia quedó `state:"open"`. Con eso, la **Fase 12 del plan original** (criterio de aceptación humano pendiente desde el inicio del proyecto) queda satisfecha: se confirmó envío y recepción real de WhatsApp (texto y PDF) contra un número real, no solo contra Evolution en modo stub.

### 22.2 Dashboard personalizable (drag-and-drop)
`web/src/pages/DashboardBuilderPage.tsx` (ruta `/dashboard-builder`, sidebar "Mi dashboard"). Reusa `@dnd-kit` (ya era dependencia, de Kanban) — cero deps nuevas. Catálogo de 15 widgets (`web/src/dashboard/widgetRegistry.tsx`: KPIs + gráficos + tabla, mismos datos que Ejecutivo/Reportes) que el usuario arma a su gusto: arrastrar para reordenar, ciclar tamaño (sm/md/lg), agregar/quitar desde un catálogo. Layout persistido en `localStorage` (reusa `storageService.ts`, que antes no se usaba en ningún lado).

### 22.3 Bot de WhatsApp — comando `reporte` (texto + PDF)
- Nueva dependencia (con permiso explícito del usuario): `github.com/go-pdf/fpdf`, única librería nueva agregada en toda la sesión.
- `api-go/internal/report/report.go`: `Text()` y `PDF()`, ambos arman el resumen ejecutivo desde `store.ExecutiveSummary` (mismos datos que `/api/reports/executive`, nada inventado).
- Comando `reporte`/`reportes` reconocido en cualquier momento por el bot (no requiere estar en el menú) — manda el texto y el PDF como adjunto real.
- Verificado real: envío confirmado por código de salida `SendMessage error: <nil>` + `SendDocument error: <nil>` contra +51 992 621 314 vía un programa Go descartable (`cmd/sendtest`, borrado después de usarlo).

### 22.4 Bot de WhatsApp — registro de incidentes con fotos/audio (a pedido del usuario, reemplaza el "no puedo procesarlo" original)
- Flujo conversacional de 2 pasos: cualquier mensaje de un número sin tickets (o la palabra clave "incidente"/"problema"/"reportar" de alguien que sí tiene) dispara "contame qué pasó" → la siguiente respuesta crea el ticket (`channel:"whatsapp"`, ahora sí poblado desde este canal) → pregunta por foto/nota de voz, acepta varias o "listo" para cerrar.
- Descarga real de medios: `evolution.Client.GetMediaBase64()` — investigado directo del código fuente de Evolution API en GitHub (`getBase64FromMediaMessage`, `POST /chat/getBase64FromMediaMessage/{instance}`) para no adivinar el contrato mal.
- Nueva tabla `attachments` (SQLite) + archivos guardados en disco bajo `/data/attachments/<ticket_id>/` (mismo volumen que ya existía, sin volumen nuevo).
- **Transcripción de audio**: `internal/gemini/client.go` ganó `Transcribe()` — intenta con el modelo configurado en `/settings` (flash-lite), si falla por cuota (HTTP 429) reintenta con un modelo Gemma (`GEMINI_FALLBACK_API_URL`), a pedido explícito del usuario. La key la carga el usuario mismo desde `/settings`, nunca se guardó en código/env.
- Adjuntos expuestos vía `GET /api/attachments?ticketId=` (nota: NO `/api/tickets/{id}/attachments` — ese patrón choca con `/api/tickets/email/{email}` en `net/http.ServeMux` de Go 1.22+, causaba panic al arrancar, detectado y corregido) y `GET /api/attachments/{id}/file`. Sección "Adjuntos" nueva en `TicketDetailPage.tsx` (web/), tarjetas de foto/audio con transcripción visible, etiqueta "Chat de WhatsApp" — mismo patrón visual que el usuario mostró como referencia.
- Verificado real end-to-end: 2 mensajes reales enviados al bot (trigger + descripción) crearon un ticket real (`channel:"whatsapp"`) contra el número autorizado.

### 22.5 Cheat sheet Qwik (nuevo servicio, :8086)
A pedido del usuario ("diseñá una URL en Qwik con todos los links y credenciales"), delegado a un agente en paralelo mientras se trabajaba en WhatsApp. `qwik-frontend/` nuevo (scaffold Qwik + adapter estático, `node:22-alpine` → `nginx:alpine`, mismo patrón que `astro-frontend`), servicio nuevo en `docker-compose.yml` (puerto 8086). Página única con: warning de seguridad de WhatsApp (número autorizado, prominente), y tarjetas agrupadas por categoría (App interna, Portal público, WhatsApp, Analítica, Admin/CMS, Backend) con URL + credenciales de cada uno de los 11 servicios reales — verificadas contra las fuentes (`authService.ts`, `docker-compose.yml`, `.env`) antes de publicarlas, no copiadas a ciegas.

### 22.6 Exposición pública por túnel (para una clase) + candado de seguridad
- `cloudflared` (ya instalado en la máquina) expone `todamark-web` (:8080), `astro-frontend` (:8082) y `qwik-frontend` (:8086) — 3 túneles `trycloudflare.com`, efímeros (mueren si se corta el proceso).
- **Bug real encontrado y arreglado en el camino:** `astro-frontend` tenía el script de config en runtime (`window.__ENV`, igual que `todamark-web`) pero le faltaba el bloque `environment:` en `docker-compose.yml` para recibir `PUBLIC_API_URL` — sin eso, ese script nunca se activaba de verdad. Agregado, consistente con el patrón que ya usaba `todamark-web`.
- `todamark-web`/`astro-frontend` reapuntados a la API pública **sin rebuild** (el proyecto ya tenía scripts de runtime-config, solo hacía falta la variable de entorno correcta al levantar el contenedor).
- **Riesgo de seguridad identificado y comunicado antes de exponer:** la API no tiene auth (`REQUIRE_API_AUTH=false`) y varios endpoints permiten mandar WhatsApp real a cualquier número. El usuario, informado del riesgo, decidió explícitamente exponer igual.
- **Bug encontrado en vivo:** "Entrar con SSO" en `/login` no llevaba a ningún lado para los alumnos — Hydra (`localhost:4444`) nunca se tuneleó, y la UI no tenía forma de saltar al login manual cuando Hydra está configurado. Arreglado con un toggle "O entrar con usuario y contraseña" en `LoginPage.tsx` (fix mínimo, no se tocó Hydra).
- **Candado `WHATSAPP_ALLOWLIST`** (`api-go/internal/evolution/client.go`): env var opcional, si está seteada SOLO se puede mandar WhatsApp a esos números — bloquea `SendMessage`/`SendDocument` en el único choque de punto (cubre bot, `/api/notify`, `/api/messages`, todo). Activado para la sesión de túnel con los 2 números autorizados del usuario (+51 992 621 314 y +51 983 644 486). **Verificado bloqueando en producción real dos veces**: una prueba manual propia y un intento real de un tercero (probablemente un alumno) contra el bot, ambos rechazados con `"número no autorizado para esta demo"`.

### 22.7 Próximo paso real
Los túneles son efímeros — si el usuario cierra la sesión de `cloudflared`, las URLs públicas mueren (los servicios locales siguen andando normal). `WHATSAPP_ALLOWLIST` no quedó hardcodeada en `docker-compose.yml` (default vacío) — si se vuelve a levantar `todomark-api` sin pasar esa variable explícita, el candado se desactiva solo (comportamiento esperado, no un bug).
