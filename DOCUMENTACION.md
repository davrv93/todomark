# TodoMark — Documentación consolidada

> Fusiona en un solo archivo: `REGLAS_NEGOCIO_Y_REPORTES.md`, `2da fase.md`, `clickhouse_reportes.md`, `plan_trabajo_clickhouse.md`, `plan_trabajo_website.md` (2026-09-23). Los 5 originales se eliminaron tras esta fusión — nada de su contenido se perdió, ver secciones abajo.
>
> **No incluidos a propósito** (siguen aparte, son archivos que el tooling carga automáticamente o que gobiernan el flujo de trabajo, no son "docs" sueltos): `AGENTS.md` (raíz y `astro-frontend/`), `PLAN.md` (plan de trabajo vivo, `AGENTS.md` exige leerlo por nombre en cada sesión), `astro-frontend/CLAUDE.md`, `astro-frontend/README.md`, y la skill `.claude/skills/todomark-design/SKILL.md`.

---

# Parte 1 — Reglas de negocio, landing, servicios y reportes

*(contenido íntegro de `REGLAS_NEGOCIO_Y_REPORTES.md`, generado 2026-09-23)*

## 1. Qué es TodoMark

Sistema de gestión de tickets de soporte (estilo GLPI), con canal WhatsApp real, sincronización con GLPI externo, portal interno para agentes y un portal público (landing) con chatbot IA y captura de leads.

## 2. Reglas de negocio

### 2.1 Ciclo de vida del ticket

Estados: `open`, `in_progress`, `resolved`, `closed`, `reopened`.

Transiciones válidas (`web/src/models/transitions.ts`, espejadas en `api-go/internal/store`):

```
open        → in_progress, closed
in_progress → resolved, open
resolved    → closed, reopened
closed      → reopened
reopened    → in_progress, closed
```

Una transición inválida se rechaza también en backend (`409`), no solo en la UI.

### 2.2 Prioridad

`low`, `medium`, `high`, `critical`. **SLA demo adoptado 2026-09-23** (proyecto demostrativo, sin gerente real que confirme umbrales todavía): crítico ≤ 4h, alta ≤ 24h, media ≤ 72h, baja ≤ 168h. Constante Go `slaTargetHoursDemo` en `api-go/internal/store/store.go`, NO es una columna de negocio confirmada — ver §13/§14.

### 2.3 Roles (RBAC)

`admin`, `agent`, `viewer` — permisos por scope vía `withAuth`/`hasRole` en `api-go`. Login interno: credenciales hardcodeadas (`admin`/`user`) o SSO real vía Hydra (OAuth2/OIDC) si `VITE_HYDRA_*` está configurado.

### 2.4 Canales de entrada de un ticket

| Canal | Cómo entra |
|---|---|
| Web interno | Formulario en `TicketCreatePage`, marca `channel:"web"` |
| WhatsApp | Webhook Evolution API → parseo de mensaje → bot con menú (`menu`/`1`-`5`) solo responde a números ya registrados como destinatarios de un ticket. No crea tickets nuevos (solo responde sobre los existentes). **Comando `reporte`** (2026-09-23): funciona en cualquier momento (no requiere estar en el menú), manda el resumen ejecutivo como texto + PDF adjunto (`internal/report`, mismos datos que `GET /api/reports/executive`, nada inventado). Sigue el mismo gate que el resto del bot: solo números que ya tienen algún ticket vinculado. |
| Email | Webhook `/webhook/email` con idempotencia (`processed_events`), marca `channel:"email"` |
| GLPI | Sync cada 5 min (`GET`/sesión GLPI), conflictos registrados en `glpi_conflicts`, marca `channel:"glpi"` |

Tickets creados antes de 2026-09-23 (antes de trackear canal) quedan como `channel:"sin_canal"` — dato honesto, no retroactivo inventado.

### 2.5 Destinatario WhatsApp

Cada ticket puede tener un `whatsappChatId` (número). Se fija desde Crear/Editar/AssignPhone/PhoneLink. Es lo que activa el bot conversacional para ese número.

**Regla de seguridad activa en esta cuenta:** ningún mensaje de prueba de WhatsApp se envía a números inventados — el único número autorizado para pruebas es **+51 992 621 314**.

## 3. Landing page (marketing)

- URL: `http://localhost:8082/landing` (servicio `astro-frontend`).
- Secciones: hero, beneficios (5 items), casos de éxito/características (parcial), formulario de lead (nombre/email/empresa) → `POST /api/leads` (persiste en SQLite, tabla `leads`, sin CRM externo conectado), chatbot flotante.
- **CMS:** Directus (`http://localhost:8083`), colección `landing_content` (`key`/`value`), editable sin tocar código. Fetch en build-time con fallback hardcodeado si Directus no responde; publicar cambios requiere rebuild de `astro-frontend` (deuda conocida, ver `PLAN.md` §20.6).
- **Chatbot:** Gemini Flash Lite (motor principal) + DeepSeek (respaldo) + clasificador de intención (`demo`/`soporte`/`queja`/`otro`, por prompt) + RAG casero (`knowledge_chunks` en SQLite, búsqueda por palabra clave, sin vector store). Cada intercambio se guarda como chunk nuevo — así "aprende" con el uso, no por reentrenamiento.
- **Keys de IA:** configurables en caliente desde `/settings` (app interna), sin editar `.env` ni reiniciar contenedor.
- **Portal de cliente final:** `http://localhost:8082/mis-tickets` — el cliente pone su email, ve sus propios tickets: totales, abiertos/resueltos, tiempo medio de resolución, gráfico por estado/prioridad, últimos tickets. Reusa `GET /api/tickets/email/{email}` (ya existía), sin login.
- **Vista gerencial y reportes operativos (2026-09-23, integrados en la app interna):** `http://localhost:8080/executive` y `http://localhost:8080/reports` — dentro de `web/` (React SPA con `Sidebar`/`Layout`/login, F5 persiste por `try_files ... /index.html` en nginx). Antes vivían solo en `astro-frontend` (:8082, `/ejecutivo` y `/`, sin sidebar); esas versiones siguen existiendo mientras no se decida sacarlas, pero la fuente principal para uso diario ahora es `web/`.

## 4. Servicios (docker-compose.yml)

| Servicio | Puerto | Qué es | Estado |
|---|---|---|---|
| `todamark-web` | 8080 | App interna (React), gestión de tickets, kanban, reportes, settings | ✅ |
| `todomark-api` | 8081 | Gateway Go: tickets, notify, whatsapp, chat, leads, settings, reportes, webhooks | ✅ |
| `evolution` | 3100 | Evolution API (WhatsApp real), imagen oficial | ✅ desplegado, ⬜ QR sin escanear con teléfono físico |
| `postgres` | interno | DB de Evolution, Hydra y Directus | ✅ |
| `clickhouse` | 8123 / 9000 | Analítica columnar, `fact_tickets`/`fact_events` sincronizando cada 5 min | 🔶 sin dashboards |
| `clickhouse-cors-proxy` | 8085 | nginx: agrega CORS limpio delante de ClickHouse (Tabix lo necesita) | ✅ |
| `tabix` | 8084 | UI web para consultar ClickHouse a mano | ✅ |
| `hydra` + `hydra-migrate` | 4444 / 4445 | OAuth2/OIDC (SSO) | ✅ |
| `astro-frontend` | 8082 | Landing pública + Kanban/Reportes (islands) + `/mis-tickets` + `/ejecutivo` (versión original, ver nota abajo) | ✅ |
| `directus` | 8083 | CMS headless para contenido de landing | ✅ (credenciales dev, rotar antes de producción) |
| `grafana` | 3300 | Dashboards sobre ClickHouse (F5), datasource+dashboard provisionados como código | ✅ (2026-09-23) login `admin`/`todomark_admin_2026` |

> **Nota (2026-09-23):** Ejecutivo/Reportes ahora viven principalmente en `web/` (`:8080/executive`, `:8080/reports`, dentro del sidebar interno con login). Las versiones en `astro-frontend` (`:8082/ejecutivo`, `:8082/`) siguen existiendo sin borrar, decisión pendiente del usuario sobre si conservarlas.

## 5. Estatus general (resumen de `PLAN.md`)

| Fase | Contenido | Estado |
|---|---|---|
| 1-14 | Scaffold, modelos, CRUD, login, WhatsApp real, destinatario, rediseño UI, sesión WhatsApp | ✅ |
| 15 | Rediseño Apple-minimal + motion | ✅ |
| 16 | Núcleo + canales (email/WhatsApp) + GLPI + RBAC hardening | ✅ |
| 17 | SSO Hydra + Astro Islands | ✅ |
| 18 | Bandeja de mensajes + presencia en vivo (SSE) | ✅ |
| 19 | Analítica ClickHouse | 🔶 ETL real, sin dbt/GraphQL/dashboards |
| 20 | Landing + CMS Directus + chatbot RAG | 🔶 real y verificado, faltan SEO y keys de IA reales |

**Pendiente humano real:** escanear el QR de WhatsApp con un teléfono físico para confirmar envío/recepción real.

## 6. Reportes que YA existen

| Reporte | Dónde | Fuente |
|---|---|---|
| Resumen operativo global (por estado, prioridad, canal, tickets/día, tiempo medio de resolución) | `todamark-web` interno + `astro-frontend` (`/`, Kanban+Reportes) | `GET /api/reports/summary` |
| Resumen ejecutivo (abiertos, cambio neto 7d, MTTR, tasa de reapertura) | `/ejecutivo` | `GET /api/reports/executive` |
| Reporte por cliente (mismo desglose, filtrado por email) | `/mis-tickets` | `GET /api/tickets/email/{email}` (cálculo en el navegador) |
| Datos crudos en ClickHouse (`fact_tickets`, `fact_events`) | Tabix (`:8084`), consulta SQL manual | Sync cada 5 min desde SQLite |

Todos honestos: ningún número inventado, todo viene de datos reales del store.

## 7. Catálogo completo de reportes — propuesta (2026-09-23, ampliación)

> Todo lo de §7 en adelante es **propuesta para F0 (discovery con el gerente)**, salvo lo ya marcado "ya existe" en §6. Nada de esto tiene endpoint ni dato real todavía salvo lo listado en §6.

### 7.1 Prompt maestro (para retomar este trabajo con cualquier IA/dev)

> Contexto: TodoMark, sistema de incidencias para una inmobiliaria sobre un ERP propio. Tickets entran por WhatsApp (Evolution API), email, web interno y sync GLPI. Actores: inquilinos (reportan incidencias de su unidad), gerentes/dueños de edificios (quieren saber si el soporte funciona y cuánto cuesta), agentes internos (resuelven). Backend Go (`api-go`, :8081), front interno React (:8080), landing + `/mis-tickets` + `/ejecutivo` Astro (:8082), analítica ClickHouse (:8123) con sync cada 5 min desde SQLite.
>
> Construir: (1) set de reportes en 4 capas — operativos, gerenciales, usuario final, analíticos/inferencia; (2) por reporte: métrica, audiencia, fuente de datos, contrato API (`GET /api/reports/...`), gráfico recomendado; (3) dashboard ejecutivo de una pantalla (4-6 tarjetas + máx. 2 gráficos, sin tablas densas), reutilizando el patrón visual de `/mis-tickets` y `/ejecutivo`; (4) sistema de componentes reutilizables (Card, KPI, TrendChart, Sparkline, Heatmap, Funnel, TableLite); (5) extensión de modelo de datos (SLA, canal —ya existe—, edificio/unidad, root cause, satisfacción) con migraciones; (6) capa analítica ClickHouse (vistas materializadas, inferencia ligera si aporta).
>
> Restricciones: honestidad de datos (nada inventado), sin dashboards vacíos, cada número trazable a SQLite/ClickHouse real, reusar RBAC (admin/agent/viewer).

### 7.2 Taxonomía de reportes (4 capas)

#### Capa 1 — Operativos (agentes + supervisor, vista interna, tiempo real)

| # | Reporte | Métrica principal | Gráfico | Fuente |
|---|---|---|---|---|
| O1 | Cola en vivo | aging por estado (horas desde que entró a ese estado, usando el último evento `history` que llevó a ese estado, o `created_at` si no hay) | barras | **ya existe**: `agingHoursByStatus` en `GET /api/reports/summary`, chart "Aging por estado" en `ReportsIsland` |
| O2 | Carga por agente | tickets abiertos por agente, WIP | barras horizontales | **ya existe**: `byAgent` en `GET /api/reports/summary`, chart "Carga por agente" en `ReportsIsland` |
| O3 | Sin asignar | count + antigüedad máx. | tarjeta | **ya existe**: `unassignedCount`/`unassignedOldestHours` en `GET /api/reports/summary` |
| O4 | Reaperturas del día | count (eventos `to:"reopened"` con timestamp de hoy) | tarjeta | **ya existe**: `reopensToday` en `GET /api/reports/summary`. Top causas NO implementado — no hay campo "motivo" en el modelo |
| O5 | SLA en riesgo | tickets abiertos con <2h para vencer el umbral demo por prioridad, y vencidos | 2 tarjetas | **ya existe**: `slaAtRiskCount`/`slaOverdueCount` en `GET /api/reports/summary` (mismo umbral demo que G3) |
| O6 | Primera respuesta | FRT = horas desde `created_at` hasta el primer evento `history` con `action:"status_change"` (proxy honesto de "un agente tocó el ticket"; no hay campo `first_response_at` real) | tarjeta | **ya existe (proxy)**: `frtHours` en `GET /api/reports/summary` |
| O7 | Fuente de entrada hoy | WhatsApp / ERP / email / web | donut | **ya existe**: `byChannel` en `GET /api/reports/summary`, donut "Por canal de entrada" en `ReportsIsland` |
| O8 | Backlog semanal | creados vs cerrados, últimas 8 semanas (lunes como inicio de semana) | línea doble | **ya existe**: `weeklyBacklog` en `GET /api/reports/summary`, mismo dato que G5 |

#### Capa 2 — Gerenciales (gerente/dueño, una pantalla)

| # | Reporte | Métrica | Gráfico | Notas |
|---|---|---|---|---|
| G1 | Tickets abiertos ahora | número grande | KPI | **ya existe** en `/ejecutivo` |
| G2 | MTTR | global | KPI | **ya existe** en `/ejecutivo` |
| G3 | % cumplimiento SLA | global | KPI | **ya existe (demo)**: `slaCompliancePct` en `/ejecutivo`, umbrales demo no confirmados por gerente real |
| G4 | Tasa de reapertura | % sobre cerrados | KPI | **ya existe** en `/ejecutivo` |
| G5 | Tendencia volumen | creados vs cerrados semanal, últimas 8 semanas | línea doble | **ya existe**: `weeklyTrend` en `/ejecutivo` |
| G6 | Top edificios con incidencias | ranking por volumen y MTTR | tabla | **forzado con datos DEMO** (autorizado por el usuario 2026-09-23): 3 edificios ficticios sembrados (`buildings`), `byBuilding` en `GET /api/reports/summary`, tabla en `ReportsIsland` |
| G7 | Top tipo de incidencia por edificio | heatmap edificio × categoría | tabla (no heatmap visual, no hay lib de heatmap en el stack) | **forzado con datos DEMO**: `categoryByBuilding`, tabla en `ReportsIsland` |
| G8 | Costo estimado de soporte | tickets × tiempo × tarifa | KPI | **forzado con datos DEMO**: `costoHoraDemo = $25/h` (constante Go, NO tarifa real), `costEstimateDemo` en `/ejecutivo` |
| G9 | Cumplimiento por proveedor/contratista | MTTR por proveedor | barras | **no forzado a propósito** — no es un dato "desconocido que se puede inventar", es un concepto que no existe en el dominio (`Ticket` no modela proveedores externos, solo agentes internos vía `assignedTo`/`assignedTeam`, ya cubierto por O2). Forzarlo sería inventar una entidad de negocio nueva, no solo un valor demo |
| G10 | Satisfacción (CSAT) | promedio post-cierre | KPI | **ya existe**: `csatAvg` en `/ejecutivo`, calificación 1-5 en `/mis-tickets` |
| G11 | Embudo comercial | visitas landing → leads → demo | 2 pasos: visitas → leads | **forzado, instrumentado real** (no inventado): `POST /api/landing/visit` (ping simple, sin cookies/terceros) desde `landing.astro`, `landing_visits`/`CountLeads` en store, expuesto en `/ejecutivo`. Trackeo empieza 2026-09-23 — el número de visitas es real desde hoy, no histórico inventado |

#### Capa 3 — Usuario final (inquilino, `/mis-tickets`)

| # | Reporte | Métrica | Gráfico |
|---|---|---|---|
| U1 | Mis tickets | totales, abiertos, resueltos | tarjetas + lista (**ya existe**) |
| U2 | Mi unidad | edificio + unidad del ticket | inline, en la fila del ticket | **forzado con datos DEMO**: `buildingId`/`unitId` en `Ticket`, selector de edificio + campo de unidad en `TicketCreatePage` (web/), mostrado en `/mis-tickets` |
| U3 | Tiempo medio de resolución de MI edificio | horas, comparado contra `byBuilding` | inline, en la fila del ticket | **forzado con datos DEMO**: `/mis-tickets` cruza el `buildingId` del ticket contra `GET /api/reports/summary`'s `byBuilding` |
| U4 | Historial por categoría | incidencias por tipo | donut — **campo ya existe** (`category`, taxonomía inferida: plomería/electricidad/mantenimiento/limpieza/seguridad/administrativo/otro, selector en `TicketCreatePage`), reporte global en `byCategory`. Vista por-cliente en `/mis-tickets` todavía no lo desglosa (pendiente, bajo esfuerzo) |
| U5 | Escalar a gerente | botón + estado escalado | CTA — **ya existe**: `POST /api/tickets/{id}/escalate`, botón en `/mis-tickets` (registra evento en `history`, sin notificación externa todavía) |
| U6 | Satisfacción post-cierre | estrellas 1-5 | inline — **ya existe** en `/mis-tickets` |

#### Capa 4 — Analíticos / Inferencia (supervisor y gerente avanzado)

| # | Reporte | Técnica | Salida |
|---|---|---|---|
| A1 | Incidencias recurrentes | clustering por texto (TF-IDF + kmeans o embeddings) | "5 patrones explican 40% del volumen" |
| A2 | Predicción de volumen | serie temporal (Prophet / ARIMA simple) | forecast 14d por edificio |
| A3 | Detección de anomalías | z-score / IQR sobre volumen diario | alerta "pico anómalo en edificio X" |
| A4 | Clasificación automática | LLM o reglas sobre texto | asigna categoría/severidad al crear |
| A5 | Score de reapertura | logística simple (canal, categoría, agente, tiempo) | % prob. de reapertura por ticket |
| A6 | Root cause por módulo ERP | correlación ticket ↔ módulo ERP | "60% de bugs vienen de Cobros" |
| A7 | Tickets duplicados | similitud semántica > umbral | sugerencia de merge |
| A8 | Priorización inteligente | score = f(severidad, antigüedad, edificio VIP, SLA) | cola reordenada |
| A9 | Correlación edificio ↔ tipo bug | matriz de co-ocurrencia | heatmap |
| A10 | Uso y efectividad del chatbot | conversaciones resueltas sin agente, categoría, tiempo | funnel + KPI |

> **Nota (2026-09-23):** A1/A2/A5/A7/A8/A9 (Fase F8, inferencia ML) requieren >500 tickets según el propio plan original (§8 más abajo). Hoy la base tiene ~6 tickets — no iniciar F8 todavía, quedaría entrenando sobre ruido.

### 7.3 Formato ideal para un gerente

Dashboard ejecutivo de una sola pantalla: 4-6 tarjetas de número grande arriba + máx. 2 gráficos de tendencia, sin tablas densas, cada número con Δ vs. periodo anterior. `/ejecutivo` y `/mis-tickets` ya siguen ese patrón.

## 8. Modelo de datos a extender (propuesto, pendiente de F0)

```sql
-- tickets: channel, satisfaction_score, category, building_id, unit_id YA implementados
-- (ver §2.4, §7.2 O7/G10/U6/U2-U3). sla_target_hours NO es columna: es constante Go por
-- prioridad (slaTargetHoursDemo en store.go).
ALTER TABLE tickets ADD COLUMN first_response_at DATETIME;  -- pendiente, O6 usa proxy (primer status_change)
ALTER TABLE tickets ADD COLUMN root_cause_module TEXT;      -- módulo ERP, pendiente (A6, no antes de F7)
ALTER TABLE tickets ADD COLUMN predicted_reopen_score REAL; -- A5, no antes de F8
ALTER TABLE tickets ADD COLUMN priority_score REAL;         -- A8, no antes de F8

-- buildings YA implementada (3 filas DEMO sembradas en migrate(), ver §13):
-- buildings(id, name, tier)  -- sin address/owner_id todavía, no hacían falta para G6/G7/U2/U3
-- landing_visits(day, count) YA implementada — contador real de G11, sin PII.

-- pendiente (bloqueado de verdad, no forzado): modelar tenant_id/units como tabla separada
-- con relación 1:N real. Hoy unit_id es texto libre en el ticket, no hay tabla `units` ni `tenants`.
tenants(id, name, email, phone)
units(id, building_id, code, tenant_id)
sla_policies(priority, target_hours, business_hours_only)
```

Regla de negocio nueva: **SLA por prioridad** — sugerido crítico < 4h, alta < 24h, media < 72h, baja < 7d. Sin esto, %SLA no es calculable con datos reales.

## 9. Contratos API sugeridos (propuesto)

```
GET  /api/reports/executive           → ya existe (G1/G2/G3-demo/G4/G10)
GET  /api/reports/summary             → ya existe (byStatus/byPriority/byChannel/byAgent/dailyTickets/averageResolutionHours)
GET  /api/reports/operational         → O1, O3-O6, O8 — pendiente (O2/O7 ya viven en /api/reports/summary)
GET  /api/buildings                   → ya existe (catálogo de 3 edificios demo)
GET  /api/reports/analytics/clusters  → A1 — no antes de F8
GET  /api/reports/analytics/forecast  → A2 — no antes de F8
GET  /api/reports/analytics/anomalies → A3 — pendiente
GET  /api/reports/analytics/root-cause→ A6 — pendiente
GET  /api/tickets/email/{email}       → ya existe, reusado en U*
POST /api/tickets/{id}/satisfaction   → ya existe (U6/G10)
POST /api/tickets/{id}/escalate       → ya existe (U5) — solo registra evento en history, sin notificación externa
POST /api/leads                       → ya existe, alimenta G11
POST /api/landing/visit               → ya existe, alimenta G11 (contador simple, sin cookies)
```

Todo detrás de `withAuth` + `hasRole`: `viewer` → solo U*; `agent` → O* + U* propios; `admin` → todo.

## 10. UI/UX — guía visual (propuesta)

**Principios:** 1 pantalla = 1 decisión. 4-6 KPIs grandes arriba, máx. 2 gráficos de tendencia, cero tablas densas. Cada número con Δ vs. periodo anterior. Todo gráfico con tooltip, export PNG, link al detalle.

**Sistema de componentes (propuesto, sin implementar como librería compartida todavía — hoy cada island repite el patrón inline):**
```
<CardKpi title value delta trend sparkline />
<TrendChart type="line|bar|area" series />
<Heatmap rows cols values />
<Funnel steps />
<Gauge value target />
<TableLite columns rows compact />
<AnomalyBadge severity />
```

**Paleta** (alineada con `theme.css`, Fase 15 Apple-minimal): fondo `#FAFAFA`, superficie `#FFFFFF`, borde `#E5E5E5`, texto `#1D1D1F`, texto secundario `#6E6E73`, acento `#0A84FF`, éxito `#30D158`, alerta `#FF9F0A`, crítico `#FF453A`. **No hardcodear estos hex sueltos al implementar — mapearlos a variables nuevas en `theme.css`.**

**Librerías:** `recharts`/`chart.js` (ya en uso) para lo simple; `visx` o `nivo` solo si se necesita heatmap/funnel más fino (dependencia nueva → pedir permiso explícito antes de instalar).

**Wireframe dashboard ejecutivo (una pantalla):**
```
┌────────────────────────────────────────────────────────────┐
│  TodoMark · Vista Gerencial          [30d ▾] [Edificio ▾] │
├─────────────┬─────────────┬─────────────┬──────────────────┤
│ Abiertos    │ MTTR        │ %SLA        │ Reaperturas      │
│ 47 ▲12%     │ 18h ▼3h     │ 92% ▲4pp    │ 6.1% ▼0.8pp      │
│ ▁▂▃▅▆▇█     │ ▇▆▅▄▃▂▁     │ ▁▂▃▄▅▆▇     │ ▇▇▆▅▄▃▂          │
├─────────────┴─────────────┴─────────────┴──────────────────┤
│  Volumen creados vs cerrados (semanal)                     │
│  ╱╲╱╲╱╲   ── creados  ── cerrados                          │
├────────────────────────────────────────────────────────────┤
│  Top 5 edificios con incidencias                           │
│  ████████████ Torre Norte     42                           │
│  ██████████   Edificio Sur    36                           │
│  ████████     Res. Central    29                           │
└────────────────────────────────────────────────────────────┘
```
(`/ejecutivo` hoy implementa las 4 tarjetas superiores sin %SLA; el resto del wireframe sigue pendiente.)

## 11. Plan de trabajo por fases (estado 2026-09-23)

| Fase | Entregable | Esfuerzo | Estado |
|---|---|---|---|
| F0 · Discovery | Definir con el gerente real: umbral SLA (adoptado umbral demo mientras tanto), tarifa/hora, edificios VIP | 1 reunión | ⬜ sigue bloqueando G6-G9/buildings/costos — SLA (G3) desbloqueado con defaults demo 2026-09-23 |
| F1 · Modelo | Migraciones de campos (`channel`/`satisfaction_score`/`category`/`building_id`/`unit_id` ✅; `buildings`/`landing_visits` ✅ con datos demo; `tenant`/`units` como tabla propia pendiente, real, no forzado) | 2-3 d | ✅ todo lo forzable ya está, solo falta el modelo relacional real de tenants/units |
| F2 · Reportes operativos | O1-O8 en front interno + endpoints Go | 4-5 d | ✅ completo |
| F3 · Dashboard ejecutivo | G1-G6, reutiliza patrón `/mis-tickets` | 3-4 d | ✅ completo (G1-G6, G6 con datos demo) |
| F4 · Reportes usuario final | U1-U6, mejora `/mis-tickets` | 2-3 d | ✅ completo (todos implementados; U2/U3 con datos demo) |
| F5 · Analítica base ClickHouse | Vistas + dashboard Grafana | 4-6 d | ✅ (2026-09-23) `fact_tickets` sincroniza `channel`/`category`/`building_id`/`satisfaction_score`; 7 vistas SQL (`v_tickets_by_status/priority/channel/category/building`, `v_mttr_by_priority`, `v_weekly_trend`, `v_csat`); Grafana provisionado (datasource + dashboard "TodoMark — Ejecutivo", 10 paneles), sin dbt/GraphQL (no pedidos) |
| F6 · Gerencial avanzado | G7/G8/G11 forzados con demo; G9 no aplica al modelo (no es "dato faltante", es concepto inexistente) | 4-5 d | ✅ G7/G8/G11; N/A G9 |
| F7 · Inferencia ligera | A3 (anomalías), A4 (clasificación), A6 (root cause) | 5-7 d | ⬜ |
| F8 · Inferencia ML | A1 (clustering), A2 (forecast), A5 (reapertura), A7 (duplicados) | 8-12 d | ⬜ no iniciar con <500 tickets |
| F9 · Pulido UI | Motion, accesibilidad, dark mode, export PDF | 3-4 d | ⬜ |

**Camino crítico:** F0 → F1 → F3 → F6 → F8.

## 12. Glosario de métricas

- **MTTR:** media de `resolved_at`/`closed_at` menos `created_at`, en horas.
- **FRT (First Response Time):** horas desde `created_at` hasta el primer evento `history` con `action:"status_change"` — proxy honesto (**ya existe**, `frtHours`); no hay columna `first_response_at` real todavía.
- **%SLA:** tickets cerrados dentro del umbral demo por prioridad (`slaTargetHoursDemo`: crítico 4h/alta 24h/media 72h/baja 168h) / total cerrados (**ya existe**, umbral NO confirmado por gerente real).
- **Tasa de reapertura:** tickets con evento `to:"reopened"` en su historial / tickets que alguna vez llegaron a resuelto o cerrado, ventana configurable (**ya existe**, todo el histórico, no solo 30d).
- **Carga por agente:** tickets abiertos/en progreso/reabiertos por `assigned_to` (**ya existe**, `byAgent`).
- **CSAT:** promedio de `satisfaction_score` (1-5) (**ya existe**).
- **Backlog neto:** creados − cerrados (**ya existe** como `netChange7d`, ventana 7 días; también como serie de 8 semanas en `weeklyBacklog`/`weeklyTrend`).
- **Aging:** horas desde que el ticket entró a su estado actual (**ya existe**, `agingHoursByStatus`, promedio por estado).

## 13. Riesgos y decisiones pendientes

| Riesgo | Mitigación |
|---|---|
| %SLA, tarifa/hora ($25) y 3 edificios son datos DEMO, no de negocio real | Etiquetado explícito "(demo)" en toda la UI (`/ejecutivo`, `ReportsIsland`, `TicketCreatePage`) + nota en `DOCUMENTACION.md`; **forzado explícitamente por el usuario 2026-09-23** ("si fuerza todo mela") porque el proyecto es demostrativo — reemplazar TODOS estos valores por datos reales antes de cualquier entrega a un gerente/cliente real |
| Tickets viejos sin `building_id`/`category` | Quedan como "sin_edificio"/"sin_categoria" (mismo patrón honesto que `channel`), no se les asigna retroactivamente un valor inventado — solo tickets nuevos (creados desde el formulario actualizado) tienen estos campos poblados |
| G9 (proveedores) no se forzó | A diferencia de SLA/tarifa/edificios (valores demo sobre un concepto que sí existe), "proveedor/contratista" es un concepto que el modelo de `Ticket` no tiene — forzarlo requeriría inventar una entidad de negocio nueva, no solo un número. Se dejó fuera explícitamente, no por omisión |
| ClickHouse sin dashboards → datos muertos | F5 obligatoria antes de F6 avanzado |
| Costos requieren tarifa → política interna | Pedir tarifa/hora en F0 o dejarla editable en `/settings` |
| Inferencia sin datos suficientes | F7/F8 solo con >500 tickets; hoy hay ~6, no iniciar |
| Duplicar `/mis-tickets`/`/ejecutivo` con futuros dashboards | Reusar los mismos componentes, no reescribir |

## 14. Próximos pasos concretos

1. Reunión F0 (1h) con un gerente real (fuera del alcance demo): mostrar el wireframe del dashboard ejecutivo (§10), confirmar o reemplazar el umbral SLA demo (§2.2), definir tarifa/hora y lista de edificios VIP.
2. Si se confirma F0 con datos reales: migración `building_id`/`unit_id`/`tenant_id` en `api-go/internal/store/store.go` (mismo patrón `ensureColumn` ya usado para `channel`/`satisfaction_score`/`category`), y mover `sla_target_hours` de constante Go a columna/tabla `sla_policies` si el negocio necesita umbrales por edificio.
3. Ya implementados sin bloqueo (2026-09-23, en dos rondas): CSAT (G10/U6), carga por agente (O2), %SLA demo (G3), escalar a gerente (U5), cola/aging (O1), sin asignar (O3), reaperturas hoy (O4), SLA en riesgo/vencido (O5), FRT proxy (O6), backlog semanal (O8), tendencia semanal (G5), categoría (`category`, taxonomía inferida, alimenta `byCategory` y U4 a nivel global).
4. Explícitamente bloqueados — requieren datos de negocio reales que no existen, no se infieren para no fabricar información: G6/G7/U2/U3 (`building_id`/`unit_id`), G8 (tarifa/hora), G11 completo (falta trackear visitas a la landing). G9 no aplica al modelo actual (no hay concepto de proveedor/contratista).
5. Fuera de alcance por diseño hasta tener más volumen: A1-A10 (inferencia/ML) — el propio plan pide >500 tickets, hoy hay ~9.
6. Siguiente candidato realista si se quiere seguir: desglose de `/mis-tickets` por categoría (U4 a nivel cliente, el campo ya existe) o instrumentar visitas en `landing.astro` (desbloquea G11).

## 15. Nota de honestidad de datos

Los campos nuevos de §8 se cargan **hacia adelante** desde que se implementan (mismo criterio ya aplicado a `channel`: tickets viejos quedan como `sin_canal`, no se inventa su canal real). Los reportes históricos por edificio/SLA/satisfacción solo serán confiables a partir de la fecha de cada migración. Comunicarlo así al gerente desde F0 — ningún reporte de este documento debe mostrarse con datos inventados o interpolados para llenar el histórico faltante.

---

# Parte 2 — Archivo histórico: Fases 2-4 (detalle original, pre-consolidación)

*(contenido íntegro de `2da fase.md`; Fases 2, 3 y 4 de este documento están ✅ implementadas y verificadas — resumen de estado consolidado en `PLAN.md` §16-§18. Se conserva acá como registro histórico detallado: tareas, criterios de aceptación, mapeo de campos, riesgos.)*

## 1. Visión general
Sistema tipo GLPI para gestión de tickets asignados a equipos o personas.
Integra canales de comunicación (correo electrónico, WhatsApp Meta API), sincroniza con GLPI externo y ofrece UI Kanban y reportes.

## 2. Requisitos funcionales
| # | Descripción |
|---|-------------|
| 1 | **Ticket**: id, título, descripción, estado, prioridad, solicitante, asignadoA (equipo/persona), whatsappChatId, email, closed_at. Operaciones CRUD vía API. Historial de eventos. |
| 2 | **Asignación**: UI para asignar ticket a equipo/usuario. Notificaciones por email y WhatsApp al asignado. |
| 3 | **Correo electrónico**: webhook inbound que crea o actualiza tickets; envío de notificaciones/outbound. |
| 4 | **Integración GLPI externa**: cliente API que sincroniza tickets bidireccionalmente. |
| 5 | **WhatsApp**: envío de mensajes mediante Meta Cloud API (plantillas aprobadas, ventana 24 h). |
| 6 | **Kanban**: vista React con columnas por estado; drag‑and‑drop para cambiar estado y actualizar backend. |
| 7 | **Reportes**: endpoint `/api/reports/summary` que devuelve métricas por estado, prioridad, tickets diarios y tiempo medio de resolución. |

## 3. Requisitos no funcionales
- **Seguridad**: autenticación OAuth2/OIDC (ver Fase 3), autorización basada en roles (admin, agente, cliente).
- **Idempotencia**: webhooks deben registrar `event_id` en tabla `processed_events` para evitar reprocesado.
- **Observabilidad**: logs estructurados (JSON), métricas Prometheus (`/metrics`), healthchecks (`/healthz`).
- **Rendimiento**: respuestas < 200 ms para consultas de tickets; limitación de carga en Kanban (paginación > 200 tickets).
- **Resiliencia**: colas/reintentos exponenciales para WhatsApp (códigos 429) y GLPI (reintentos 3×).
- **Auditoría**: registro de cambios en `ticket_history` con usuario, timestamp, acción, campos modificados.

## 4. Arquitectura propuesta
```
┌─────────────────────────────────────────────────────────────────┐
│ docker‑compose.yml                                               │
│                                                                 │
│  ┌─────────────┐   ┌───────────────┐   ┌─────────────────────┐ │
│  │ todamark‑web│←─►│ todomark‑api  │←─►│ evolution (WhatsApp)│ │
│  │ React/Vite │   │ Go + SQLite   │   │ evoapicloud/evolution│ │
│  └─────────────┘   └───────┬───────┘   └─────────────────────┘ │
│                     │                                         │
│                     ▼                                         │
│          ┌───────────────────────┐                         │
│          │ emailbridge (Go)      │  ← webhook inbound      │
│          │ POST /webhook/email   │                         │
│          └───────────────────────┘                         │
│                     ▲                                         │
│                     │ GLPI sync (goroutine)                  │
│          ┌───────────────────────┐                         │
│          │ glpi client (Go)       │                         │
│          └───────────────────────┘                         │
│                                                                 │
│  ┌─────────────────────┐   ┌───────────────────────┐          │
│  │ astro‑frontend       │   │ postgres (internal)   │          │
│  │ Astro Islands (React│   │ (solo GLPI interno)   │          │
│  │ + Kanban + Reports) │   └───────────────────────┘          │
│  └─────────────────────┘                                   │
└─────────────────────────────────────────────────────────────┘
```
- **Frontend**: React 18 + Vite, UI Kanban y Reportes.
- **Backend**: Go 1.22 con `net/http`, SQLite como origen de verdad.
- **Canales**: webhook email (multipart/form‑data), WhatsApp Meta (Cloud API), GLPI REST.
- **Extensiones**: OAuth2/OIDC con Ory Hydra, Astro Islands (Fase 3).

## 5. Fase 2 — Núcleo e integraciones base

### 5.1 2A — Núcleo tickets
| Objetivo | Proveer modelo de ticket sólido y API completa. |
|----------|-----------------------------------------------|
| Alcance | Migración DB, modelo Go, store, rutas API, Kanban UI, reportes. |
| Tareas |
| • Crear migración `20230922_add_email_team.sql` que añada columnas `email TEXT`, `assigned_team TEXT`, `closed_at DATETIME`. |
| • Extender struct `Ticket` en `api-go/internal/store/model.go` con campos `Email`, `AssignedTeam`, `ClosedAt`. |
| • Modificar `applyPatch` (líneas 525‑558) para aceptar los nuevos campos. |
| • Añadir métodos `ListByEmail(email string)`, `ListByTeam(team string)` en `store.go`. |
| • Registrar rutas en `main.go`:<br>`GET /api/tickets/email/{email}` → `store.ListByEmail`<br>`GET /api/tickets/team/{team}` → `store.ListByTeam`. |
| • Implementar UI Kanban (`src/pages/KanbanPage.tsx`) usando `@dnd-kit/core` + `@dnd-kit/sortable`. |
| • Implementar UI Reportes (`src/pages/ReportsPage.tsx`) con Chart.js. |
| • Añadir endpoint `/api/reports/summary` que ejecuta consultas SQL (incluye `closed_at`). |
| Criterios de aceptación |
| • Migración aplicada sin pérdida de datos. |
| • API CRUD funciona, incluye filtros por email y equipo. |
| • Kanban muestra columnas `open`, `in_progress`, `resolved`, `closed`; drag‑and‑drop actualiza estado vía `ticketService.transition`. |
| • Reportes devuelven JSON con métricas correctas y cálculo de tiempo medio de resolución usando `closed_at`. |
| Riesgos |
| • Cambio de schema puede romper versiones anteriores → plan de rollback. |
| • Drag‑and‑drop con React 18 requiere `@dnd-kit`; tests unitarios necesarios. |

### 5.2 2B — Canales (email & WhatsApp)
| Objetivo | Gestionar inbound/outbound por email y WhatsApp. |
|----------|-------------------------------------------------|
| Alcance | Webhook email, cliente WhatsApp Meta, notificaciones. |
| Tareas |
| • **Webhook email**:<br>Crear `api-go/internal/email/webhook.go` con handler `POST /webhook/email` que parsea `multipart/form-data` (campo `from`, `subject`, `text`). |
| • Detectar `message_id` y registrar en tabla `processed_events` para idempotencia. |
| • Lógica de creación/actualización: si `subject` contiene `[Ticket#<id>]` → actualización; si no → creación con `email` del remitente. |
| • **Cliente WhatsApp Meta**:<br>Crear paquete `api-go/internal/whatsapp/client.go` que envía mensajes a `https://graph.facebook.com/v16.0/<PHONE_NUMBER_ID>/messages` con `Authorization: Bearer <META_WHATSAPP_TOKEN>`. |
| • Implementar gestión de plantillas aprobadas y ventana de 24 h. |
| • Manejar errores 429 mediante back‑off exponencial (retry ≤ 3). |
| • Añadir variable de entorno `USE_META_WHATSAPP` para habilitar/inhabilitar cliente. |
| Criterios de aceptación |
| • Webhook email responde 200 inmediatamente y procesa mensaje una sola vez. |
| • Mensaje enviado a WhatsApp llega correctamente; errores 429 se reintentan según política. |
| • Notificaciones por email se envían vía `notificationService.sendEmail` (wrapper a SendGrid/Mailgun). |
| Riesgos |
| • Formato multipart puede variar entre proveedores; necesidad de pruebas con SendGrid, Mailgun, Postmark. |
| • Límite de 24 h y necesidad de plantillas pueden causar rechazos; gestión de plantillas requerida. |

### 5.3 2C — Interoperabilidad GLPI
| Objetivo | Sincronizar tickets con GLPI externo. |
|----------|---------------------------------------|
| Alcance | Cliente API, mapeo de campos, sincronización periódica, resolución de conflictos. |
| Tareas |
| • Crear paquete `api-go/internal/glpi/client.go`. |
| • Variables de entorno: `GLPI_URL`, `GLPI_APP_TOKEN`, `GLPI_USER`, `GLPI_PASSWORD`. |
| • Implementar flujo de sesión GLPI:<br>`initSession` → obtener `session_token` → usar en cada request.<br>`killSession` al shutdown. |
| • Operaciones API: `GET /apirest.php/<itemtype>` (GET tickets), `POST` (crear), `PUT` (actualizar). |
| • Mapeo de campos (tabla a continuación). |
| • Sincronizador goroutine en `main.go` que cada 5 min compara tickets locales y remotos usando `remote_id`, `last_sync_at`, `local_updated_at`, `remote_updated_at`. |
| • Detectar conflicto: diferencias en ambos lados → registrar en `glpi_conflicts` y generar alerta. |
| Criterios de aceptación |
| • Cliente GLPI puede listar, crear y actualizar tickets en GLPI externo. |
| • Sincronizador mantiene `remote_id` y `last_sync_at` actualizados; ausencia de conflictos en pruebas de integración. |
| Riesgos |
| • GLPI requiere `App-Token` y `Session-Token`; olvidar cualquiera provoca 401. |
| • Estrategia "último escrito gana" sustituida por timestamps; requiere pruebas de colisión. |

#### Mapeo de campos GLPI ↔️ modelo interno
| GLPI campo | Modelo interno | Comentario |
|------------|----------------|------------|
| `name` | `title` | Texto libre |
| `content` | `description` | Texto largo |
| `status` | `status` | Conversión tabla `glpi_status` → `TicketStatus` |
| `priority` | `priority` | Igual |
| `requester` | `requester` | Email del solicitante |
| `assigned_to` | `assigned_team` | Equipo o usuario |
| `date_creation` | `created_at` | ISO |
| `date_mod` | `updated_at` | ISO |
| `closed_at` | `closed_at` | Opcional |

### 5.4 2D — Endurecimiento (auth, idempotencia, observabilidad, pruebas, Docker)
| Objetivo | Consolidar seguridad, pruebas y despliegue. |
|----------|----------------------------------------------|
| Alcance | OAuth2/OIDC (ver Fase 3), control de permisos, idempotencia de webhooks, logs/metrics, CI/CD, Docker adjustments. |
| Tareas |
| • **Autenticación básica**: proteger todas las rutas `/api/*` con middleware que valida JWT emitido por Ory Hydra (ver Fase 3). |
| • **Permisos**: roles `admin`, `agent`, `viewer`; control mediante claims en JWT. |
| • **Idempotencia**: tabla `processed_events(event_id TEXT PRIMARY KEY, processed_at TIMESTAMP)`. Cada webhook verifica existencia antes de procesar. |
| • **Observabilidad**: añadir logger estructurado (zap) y endpoint `/metrics` (Prometheus), healthcheck `/healthz`. |
| • **Pruebas**: unitarias de store, webhook, cliente GLPI, cliente WhatsApp; integración con Docker compose (mock services). |
| • **Docker**: pasar variables `VITE_*` como `build.args` en `docker‑compose.yml`; crear `config.js` servido por nginx para runtime config fallback. |
| • **Healthchecks**: definir `healthcheck` en compose (`curl -f http://localhost:3000/healthz || exit 1`). |
| Criterios de aceptación |
| • Todas las rutas API requieren JWT válido; respuestas 401/403 cuando falte permiso. |
| • Webhook procesado una sola vez aunque se reenvíe. |
| • Logs en JSON contienen `request_id`, `user`, `action`. |
| • Métricas `/metrics` expuestas y recopilables. |
| • CI ejecuta `go test ./...`, `npm run test`, `docker compose up -d --build` sin errores. |
| Riesgos |
| • Integración con Ory Hydra implica configuración externa; se marca como "a validar" la URL de autorización. |
| • Cambiar a `build.args` puede requerir rebuild de la imagen web; documentación actualizada necesaria. |

## 6. Fase 3 — Extensiones

### 6.1 3A — OAuth2/OIDC con Ory Hydra
| Justificación | Mantener Fase 2 centrada en core funcional; OAuth agrega capa de seguridad que depende de infraestructura externa. |
|----------------|-----------------------------------------------------------------------------------|
| Alcance | Implementar login mediante flujo **Authorization Code + PKCE** usando `oidc-client-ts` y `react-oidc-context`. |
| Tareas |
| • Añadir dependencias: `npm i oidc-client-ts react-oidc-context`. |
| • Crear `src/components/HydraOAuth.tsx` que exporta `<OidcProvider>` configurado con `clientId`, `authority` (Hydra URL) y `redirectUri`. |
| • En `src/App.tsx` envolver app con `<OidcProvider>`. |
| • Implementar página `LoginPage.tsx` que inicia flujo PKCE y guarda token en `localStorage`. |
| • Backend: middleware que valida JWT contra Hydra `introspection_endpoint`. |
| Criterios de aceptación |
| • Usuario accede a `/login`, es redirigido a Hydra, regresa con código, token almacenado y rutas `/api/*` accesibles. |
| • Tokens expirados provocan 401 y redirección a login. |
| Riesgos |
| • Necesidad de certificado TLS en producción; Hydra debe estar disponible. |
| • PKCE requiere generación segura de `code_verifier`; pruebas unitarias necesarias. |

### 6.2 3B — Astro Islands (Kanban & Reportes)
| Justificación | Astro permite servir componentes React como islands, reduciendo carga inicial y aprovechando server‑side rendering. |
|----------------|-----------------------------------------------------------------------------------|
| Alcance | Integrar Kanban y Reportes como islands dentro proyecto Astro, manteniendo la UI principal en React/Vite. |
| Tareas |
| • Crear directorio `astro-frontend/`; ejecutar `npm init astro@latest` eligiendo "React". |
| • Copiar `KanbanPage.tsx` y `ReportsPage.tsx` a `astro-frontend/src/components/`. |
| • En `src/pages/index.astro` usar `<KanbanIsland client:load />` y `<ReportsIsland client:load />`. |
| • Configurar `astro.config.mjs` para exponer variables de entorno (`VITE_HYDRA_*`) como `runtimeConfig`. |
| • Añadir Dockerfile que compile Astro y sirva con Nginx. |
| • Añadir servicio `astro-frontend` en `docker-compose.yml` (ver sección Docker). |
| Criterios de aceptación |
| • Navegando a `http://localhost:8082/` se muestra Kanban y Reportes como islands, cargados de forma diferida. |
| • Performance mejora (bundle principal reduce < 500 KB). |
| Riesgos |
| • Duplicación de código React entre web y Astro; se necesita mantener consistencia. |
| • Configuración de runtime env en Astro puede ser compleja; pruebas de despliegue necesarias. |

## 7. Plan de pruebas
| Tipo | Alcance | Herramienta |
|------|---------|-------------|
| Unitarias | Store (`ListByEmail`, `ListByTeam`, `applyPatch`), webhook email idempotencia, cliente WhatsApp, cliente GLPI | `go test ./...` |
| UI Unit | `KanbanPage`, `ReportsPage`, `HydraOAuth` | React Testing Library + Jest |
| Integración | Flujo completo: crear ticket → email webhook → asignación → notificación WhatsApp → Kanban drag‑and‑drop → reporte | Docker compose con mocks (SendGrid, Meta API, GLPI) + `Testcontainers` |
| End‑to‑End | Acceso vía UI, login OIDC, operaciones CRUD, sincronización GLPI | Cypress (o Playwright) |
| Performance | Tiempo medio de respuesta < 200 ms bajo carga 100 req/s | `k6` o `hey` |
| Seguridad | Escaneo OWASP ZAP, pruebas de JWT expirado, CSRF | ZAP, manual tests |

CI pipeline:
```yaml
steps:
  - go test ./...
  - cd web && npm ci && npm test
  - cd astro-frontend && npm ci && npm test
  - npx tsc --noEmit && npm run build   # web
  - go build ./...
  - docker compose up -d --build
  - docker compose exec todamark-api go test ./...
  - docker compose exec todamark-web npm run test
```

## 8. Docker y despliegue
| Servicio | Variables (build‑time) | Variables (runtime) |
|----------|------------------------|---------------------|
| `todomark-web` | `VITE_API_URL`, `VITE_HYDRA_CLIENT_ID`, `VITE_HYDRA_AUTH_URL` (pasados como `build.args`) | N/A (servido por nginx) |
| `todomark-api` | `GLPI_URL`, `GLPI_APP_TOKEN`, `GLPI_USER`, `GLPI_PASSWORD`, `META_WHATSAPP_TOKEN`, `USE_META_WHATSAPP`, `VITE_HYDRA_CLIENT_ID`, `VITE_HYDRA_AUTH_URL` | `PORT`, `WEBHOOK_URL` |
| `astro-frontend` | `VITE_HYDRA_CLIENT_ID`, `VITE_HYDRA_AUTH_URL` (build.args) | N/A |

Ejemplo fragmento `docker‑compose.yml` (histórico — el real vive en `docker-compose.yml`, ya con 10 servicios):
```yaml
services:
  todomark-web:
    build:
      context: ./web
      args:
        VITE_API_URL: ${VITE_API_URL}
        VITE_HYDRA_CLIENT_ID: ${VITE_HYDRA_CLIENT_ID}
        VITE_HYDRA_AUTH_URL: ${VITE_HYDRA_AUTH_URL}
    ports: ["8080:80"]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3

  todomark-api:
    build: ./api-go
    environment:
      - GLPI_URL=${GLPI_URL}
      - GLPI_APP_TOKEN=${GLPI_APP_TOKEN}
      - GLPI_USER=${GLPI_USER}
      - GLPI_PASSWORD=${GLPI_PASSWORD}
      - META_WHATSAPP_TOKEN=${META_WHATSAPP_TOKEN}
      - USE_META_WHATSAPP=${USE_META_WHATSAPP}
      - VITE_HYDRA_CLIENT_ID=${VITE_HYDRA_CLIENT_ID}
      - VITE_HYDRA_AUTH_URL=${VITE_HYDRA_AUTH_URL}
    ports: ["8081:3000"]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3

  astro-frontend:
    build:
      context: ./astro-frontend
      args:
        VITE_HYDRA_CLIENT_ID: ${VITE_HYDRA_CLIENT_ID}
        VITE_HYDRA_AUTH_URL: ${VITE_HYDRA_AUTH_URL}
    ports: ["8082:80"]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost"]
      interval: 30s
      timeout: 5s
      retries: 3
```

## 9. Riesgos y mitigaciones (tabla consolidada)
| Riesgo | Impacto | Mitigación |
|--------|----------|------------|
| Incompatibilidad `@dnd-kit` con React 18 | Fallo UI Kanban | Versionar `@dnd-kit` v6, pruebas unitarias. |
| Webhook email formato inesperado | Duplicados / pérdida datos | Parser multipart genérico; pruebas con SendGrid, Mailgun, Postmark. |
| Límite 24 h y plantillas WhatsApp | Mensajes rechazados | Pre‑cargar plantillas aprobadas, fallback a notificación email. |
| GLPI requiere sesión + App‑Token | 401 inesperados | Implementar `initSession` + refresco token; pruebas de autenticación. |
| Conflictos de sincronización GLPI | Pérdida consistencia | Estrategia `remote_id` + timestamps + tabla `glpi_conflicts`. |
| Variables `VITE_*` mal pasadas | Build incorrecto | Usar `build.args` y validar en CI (`npm run build` sin env). |
| Idempotencia webhook | Duplicado de tickets | Tabla `processed_events` y clave `event_id`. |
| Falta de observabilidad | Dificultad diagnóstico | Logger JSON, Prometheus, healthchecks. |
| Dependencia externa Hydra | Bloqueo auth | Marca "a validar" la URL; tests con mock Hydra. |
| Duplicación de código entre React y Astro | Mantenimiento costoso | Compartir componentes vía monorepo, pruebas de integración. |

## 10. Próximos pasos (histórico, ya ejecutados)
1. Implementar migración DB y actualizar modelo Go.
2. Desarrollar webhook email con idempotencia.
3. Construir cliente GLPI (sesión, token, mapeo).
4. Integrar cliente WhatsApp Meta con gestión de plantillas y back‑off.
5. Implementar UI Kanban con `@dnd-kit` y UI Reportes con Chart.js.
6. Añadir endpoint `/api/reports/summary` y pruebas unitarias.
7. Hardenizar API (JWT, roles, healthchecks, logs).
8. Configurar Docker‑compose con `build.args` y healthchecks.
9. Desplegar rama `feature/kanban` y validar en entorno Docker.
10. Iniciar Fase 3: integrar Ory Hydra (OIDC) y Astro Islands en ramas `feature/hydra-oauth` y `feature/astro-islands`.
11. Ejecutar suite CI completa y aprobar criterios de aceptación.

## 11. Fase 4 — Bandeja de mensajes WhatsApp + presencia en tiempo real

> Motivo: hoy el destinatario de WhatsApp solo existe como campo de un ticket (Crear/Editar/Asignar número); no hay forma de ver conversaciones ni escribir un mensaje suelto a un número o grupo. Tampoco hay ninguna señal en vivo de quién está usando la plataforma. Fase 4 cubre ambas cosas.

### 11.1 4A — Bandeja de mensajes (inbox + compose)

| Objetivo | Ver conversaciones de WhatsApp (individuales y de grupo) y poder escribir un mensaje nuevo sin pasar por un ticket. |
|----------|-----------------------------------------------------------------------------------------------------------------|
| Alcance | Persistencia de mensajes, endpoints de hilos/envío, página de bandeja con selector Número/Grupo reutilizando `WhatsappTargetInput`. |
| Tareas |
| • Migración `api-go/internal/store/migrations/`: tabla `whatsapp_messages(id, chat_id TEXT, chat_name TEXT, direction TEXT CHECK(direction IN ('in','out')), text TEXT, ticket_id TEXT NULL, created_at DATETIME)`, índice por `chat_id`. |
| • `store.go`: `SaveMessage(msg)`, `ListThreads()` (un row por `chat_id` con último mensaje + timestamp, `ORDER BY created_at DESC`), `ListMessages(chatId, limit)`. |
| • `main.go` webhook `/webhook/evolution`: además del flujo del chatbot (que sigue igual, solo mira números registrados en tickets), persistir el mensaje entrante en `whatsapp_messages` (`direction:'in'`) — si `INBOX_SHOW_ALL=true` (default) se guarda cualquier `chat_id`; si es `false`, solo si ese `chat_id` es `whatsapp_chat_id` de algún ticket existente. |
| • Nueva ruta `POST /api/messages` `{to, text}`: llama `evo.SendMessage(to, text)` y guarda el mensaje (`direction:'out'`) — mismo envío que usa `NotifyButton`, pero sin requerir un ticket. |
| • Nuevas rutas `GET /api/messages/threads` y `GET /api/messages/threads/{chatId}`. |
| • Frontend: página nueva `web/src/pages/MessagesPage.tsx`, ruta `/messages`, entrada en `Sidebar` ("Bandeja"). Layout de dos columnas: lista de hilos (izquierda, click para seleccionar) + burbujas de mensaje y formulario de envío (derecha). El formulario de mensaje **nuevo** reusa `WhatsappTargetInput` (toggle Número/Grupo ya construido); responder dentro de un hilo ya conocido no necesita el selector, solo el chat_id ya elegido. |
| Criterios de aceptación |
| • Un mensaje entrante de un número/grupo cualquiera aparece en la bandeja sin necesidad de un ticket previo. |
| • Escribir y enviar desde la bandeja llega de verdad a WhatsApp (mismo camino que `NotifyButton`, verificado con Evolution real). |
| • Recargar la página conserva el historial (persistido en SQLite, no en memoria). |
| Riesgos |
| • Privacidad: la bandeja expone TODO lo que le llega al número de la instancia, no solo tickets — es el comportamiento esperado de un inbox, pero hay que decirlo explícito. |
| • Sin paginación real todavía: si el volumen crece, `ListMessages` necesitará límite+cursor (queda fuera de este alcance, anotar como deuda). |

### 11.2 4B — Presencia en tiempo real (SSE)

| Objetivo | Mostrar cuántos clientes están conectados a la plataforma ahora mismo. |
|----------|--------------------------------------------------------------------|
| Alcance | Endpoint WebSocket en `api-go`, contador en memoria, badge en `Header`. |
| Tareas |
| • **SSE** (`Server-Sent Events`), solo `net/http` — sin dependencias nuevas. |
| • `api-go/internal/presence/hub.go`: hub en memoria (`map[chan int]bool` + mutex), `Add()`/`Remove()`/`Count()`, broadcast del count a todos los conectados en cada cambio. |
| • Ruta `GET /api/presence/stream`: `Content-Type: text/event-stream`, suma 1 al hub, `flusher.Flush()` en cada evento, resta 1 al cerrar (`r.Context().Done()`). |
| • Frontend: hook `useConnectedCount()` (`web/src/hooks/useConnectedCount.ts`) con `EventSource`, expone el número; usado en `Header.tsx` como badge pequeño (ej. junto a la campana: "3 en línea"). |
| Criterios de aceptación |
| • Abrir la app en 2 pestañas/navegadores distintos muestra "2" en ambas, en vivo, sin recargar. |
| • Cerrar una pestaña baja el contador en la otra en segundos. |
| Riesgos |
| • Contador en memoria de un solo contenedor `todomark-api`: si se escala a 2+ réplicas del API, el conteo quedaría partido (necesitaría Redis pub/sub). No es el caso hoy, se anota como límite conocido. |
| • WS/SSE a través del puerto directo del API (`:8081`, como ya hace `fetch` con `API_URL`) no necesita tocar `nginx.conf`; si en el futuro se enruta por `todamark-web`, ahí sí hace falta `proxy_set_header Upgrade`/`Connection`. |

### 11.3 Orden de ejecución propuesto

1. 4B primero (más chico, no toca el modelo de datos, deja ver el patrón funcionando).
2. 4A después (toca DB + varias páginas nuevas).

### 11.4 Decisiones (resueltas 2026-09-23)

- [x] Presencia: **SSE**, sin dependencias nuevas (`net/http` solo).
- [x] Alcance bandeja: **configurable** — env var `INBOX_SHOW_ALL` (default `true`, mismo patrón que `USE_META_WHATSAPP`/`REQUIRE_API_AUTH`). En `true`: todo mensaje entrante se guarda y aparece en la bandeja. En `false`: solo se guardan/muestran mensajes de un `chat_id` que ya sea `whatsapp_chat_id` de algún ticket.
- [x] Ubicación: página propia **`/messages`**, entrada nueva en `Sidebar`.

---

# Parte 3 — Redirects históricos (`clickhouse_reportes.md`, `plan_trabajo_clickhouse.md`, `plan_trabajo_website.md`)

Estos 3 archivos eran ya solo punteros de 3 líneas cada uno (contenido real movido hace tiempo). Se resumen acá y se eliminan como originales:

- **`plan_trabajo_clickhouse.md` + `clickhouse_reportes.md`** → su contenido completo (arquitectura, modelo de datos, KPIs, queries, roadmap de analítica ClickHouse) vive en **`PLAN.md` §19 — Fase 5 — Analítica ClickHouse** (§19.5 queries KPI, §19.6 queries de visualización, §19.7 análisis recomendado, §19.8 roadmap por sprint). Editar solo `PLAN.md`.
- **`plan_trabajo_website.md`** → su contenido completo (visión, requisitos, arquitectura, secciones, chatbot, CMS, roadmap de la landing) vive en **`PLAN.md` §20 — Fase 6 — Landing page + chatbot + CMS**. Editar solo `PLAN.md`.

No se duplica ese contenido acá para no tener dos fuentes de verdad divergentes con `PLAN.md` — este archivo consolidado es para las Partes 1 y 2 (docs sueltas y registro histórico), no reemplaza al plan vivo.
