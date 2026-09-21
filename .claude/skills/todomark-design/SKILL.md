---
name: todomark-design
description: Design system de TodoMark (tokens, tipografía, partículas CSS, componentes de UI). Usar antes de tocar cualquier vista, componente o estilo en web/src.
---

# TodoMark — Design System

Fuente de verdad: `web/src/styles/theme.css` (importado una sola vez en `web/src/main.tsx`). No hardcodear colores, radios o sombras fuera de esas variables — si falta un token, agregarlo ahí, no inventar un valor suelto en el componente.

## Principios
- Minimalista y compacto: fuentes 12–14px en UI densa (tablas, badges, sidebar), sin paddings grandes.
- Colores "confiables": azul primario + slate neutro, sin gradientes, sin el patrón "AI" (Inter/Roboto/Arial, tarjetas con borde izquierdo de color, emojis como icono).
- Iconos: SVG inline stroke (`currentColor`, `strokeWidth` 1.9–2.2), nunca emoji.
- Responsivo: breakpoint único `@media (max-width: 860px)` en `theme.css` (sidebar colapsa a solo iconos, grids de 2 columnas pasan a 1).
- Partículas CSS (`@keyframes floatParticle` + clase `.particle`): motivo decorativo sutil, solo en zonas de marca/foco (Header, topbar móvil, card de Login). No abusar — nunca dentro de tablas o listas densas.

## Tokens clave (`theme.css`, `:root`)
- Fondo/superficie: `--bg` (#F6F7F9), `--surface` (#fff), `--surface-alt` (#F0F2F5), `--border` (#E2E5EA).
- Texto: `--ink` (principal), `--ink-soft` (secundario), `--ink-faint` (terciario/placeholder).
- Marca: `--primary` (#1E4FD8) / `--primary-dark` / `--primary-tint` (fondo claro para estados activos).
- Semánticos (estado/prioridad), cada uno con par `color` + `-tint` de fondo: `--teal` (resuelto/éxito), `--amber` (en progreso/advertencia), `--red` (reabierto/crítico/error), `--gray` (cerrado/neutro).
- Forma: `--radius` (10px, cards), `--radius-sm` (7px, botones/inputs/badges), `--shadow` (sombra sutil única del proyecto).
- Tipografía: `--font-display` (Space Grotesk — títulos `h1/h2/h3`), `--font-body` (Public Sans — todo lo demás). Cargadas vía `@import` de Google Fonts en la primera línea de `theme.css`.

## Clases utilitarias (usar, no reinventar)
- Botones: `.btn` + una de `.btn-primary` / `.btn-outline` / `.btn-outline-primary` / `.btn-ghost`.
- Inputs: `.field` (wrapper label+control) + `.input` / `.select` / `.textarea`.
- Contenedor: `.card` (superficie con borde+sombra), `.form-card` (card en columna, `max-width: 620px`, para formularios).
- Estado/prioridad: no armar badges a mano — usar `<TicketStatusBadge status={...} />` / `<TicketPriorityTag priority={...} />` de `web/src/components/TicketStatusBadge.tsx` (expone también `STATUS_META`/`PRIORITY_META` para leer label/color en otros contextos, p. ej. botones de transición en `TicketDetailPage`).
- Toggle/segmentado: `.pill` (+ `.active`) para filtros tipo chip; `.seg-btn` (+ `.active`) para selects tipo segmented control (ver prioridad en Create/Edit).
- Tabla: `.table-wrap` (contenedor con scroll) > `.ticket-table` (`thead`/`tbody` ya estilados) — ver `TicketTable.tsx`/`TicketRow.tsx`.
- Grids responsivos: `.detail-grid` (2 columnas, detalle de ticket) y `.two-col-grid` (2 columnas, pares de campos) — colapsan a 1 columna bajo 860px automáticamente, no fijar `gridTemplateColumns` inline si se puede usar estas clases.
- Texto de apoyo: `.muted`, `.error-text`, `.ok-text`.
- Avatar iniciales: `.avatar` (círculo con iniciales, usado en sidebar/tabla/detalle).

## Layout de página
`Layout.tsx`: columna `Header` (60px, full width) → fila `Sidebar` (210px, colapsa a 60px en móvil) + `<main className="page-main">`. Cualquier página nueva va dentro de `<main>` vía routing en `App.tsx`; no envolver en otro layout.

## Al agregar una vista o componente nuevo
1. Reusar `.card`/`.form-card` + clases de arriba antes de escribir CSS inline nuevo.
2. Si necesita mostrar estado o prioridad de un ticket, usar `TicketStatusBadge`/`TicketPriorityTag`, no duplicar el mapeo de colores.
3. Fechas: usar `formatRelative`/`formatDateTime` de `web/src/utils/date.ts`, no formatear a mano.
4. Si la vista es un flujo enfocado (login, formularios cortos) seguir el patrón de `LoginPage.tsx`/`AssignPhonePage.tsx`: `max-width` fijo, `.card.form-card`, breadcrumb "← Volver" con el ícono `arrow-left` inline.
5. Verificar en las dos anchuras: desktop (Layout normal) y < 860px (sidebar colapsado, grids en 1 columna).
