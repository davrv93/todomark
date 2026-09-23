-- Esquema analítico ClickHouse (PLAN.md §19.3). Ejecutado por Client con CREATE TABLE IF NOT EXISTS.
--
-- Nota de mapeo: store.Ticket no tiene entidades Cliente/Agente con ID numérico propio
-- (solo texto libre: Requester, Email, AssignedTo, AssignedTeam), así que fact_tickets
-- guarda esos campos como String en vez de inventar ClientID/AgentID falsos. Status
-- incluye 'reopened' (existe en store.Transitions) además de los 4 valores del ejemplo
-- de §19.3.

CREATE TABLE IF NOT EXISTS fact_tickets
(
    TicketID     UInt64,
    Title        String,
    Status       Enum8('open' = 1, 'in_progress' = 2, 'resolved' = 3, 'closed' = 4, 'reopened' = 5),
    Priority     Enum8('low' = 1, 'medium' = 2, 'high' = 3, 'critical' = 4),
    Requester    String,
    Email        String,
    AssignedTo   String,
    AssignedTeam String,
    CreatedAt    DateTime,
    UpdatedAt    DateTime,
    ClosedAt     Nullable(DateTime)
)
ENGINE = MergeTree()
ORDER BY (CreatedAt);

-- Agregado 2026-09-23 (F5): channel/category/building_id/satisfaction_score ya existen en
-- SQLite (store.Ticket) pero no se habían sincronizado. ALTER ... IF NOT EXISTS porque
-- CREATE TABLE IF NOT EXISTS de arriba es no-op sobre una tabla que ya existe en Docker.
ALTER TABLE fact_tickets ADD COLUMN IF NOT EXISTS Channel String DEFAULT '';
ALTER TABLE fact_tickets ADD COLUMN IF NOT EXISTS Category String DEFAULT '';
ALTER TABLE fact_tickets ADD COLUMN IF NOT EXISTS BuildingID String DEFAULT '';
ALTER TABLE fact_tickets ADD COLUMN IF NOT EXISTS SatisfactionScore Nullable(UInt8);

CREATE TABLE IF NOT EXISTS fact_events
(
    TicketID  UInt64,
    Timestamp DateTime,
    User      String,
    Action    LowCardinality(String),
    FromState String,
    ToState   String,
    CreatedAt DateTime
)
ENGINE = MergeTree()
ORDER BY (CreatedAt);

-- Sin poblar todavía: store no tiene tabla de notificaciones enviadas.
CREATE TABLE IF NOT EXISTS fact_notifications
(
    NotificationID String,
    TicketID       UInt64,
    Channel        LowCardinality(String),
    Status         LowCardinality(String),
    SentAt         DateTime,
    CreatedAt      DateTime
)
ENGINE = MergeTree()
ORDER BY (CreatedAt);

-- Dimensiones: creadas por completitud del esquema; el Syncer (sync.go) todavía no
-- las puebla (store no tiene entidades Cliente/Agente/Calendario separadas).
CREATE TABLE IF NOT EXISTS dim_client
(
    ClientID UInt64,
    Name     String,
    Email    String
)
ENGINE = MergeTree()
ORDER BY (ClientID);

CREATE TABLE IF NOT EXISTS dim_user
(
    UserID UInt64,
    Name   String,
    Team   String
)
ENGINE = MergeTree()
ORDER BY (UserID);

CREATE TABLE IF NOT EXISTS dim_time
(
    Date       Date,
    Year       UInt16,
    Quarter    UInt8,
    Month      UInt8,
    Week       UInt8,
    DayOfWeek  UInt8,
    DayOfMonth UInt8
)
ENGINE = MergeTree()
ORDER BY (Date);

-- Vistas (F5, 2026-09-23) — CREATE VIEW normal, no MATERIALIZED VIEW: el Syncer trunca y
-- reinserta fact_tickets/fact_events cada 5 min (ver sync.go), así que una MV incremental
-- (que solo reacciona a INSERT) quedaría con datos viejos después del TRUNCATE. Una vista
-- normal siempre calcula contra el estado actual de las tablas — correcto para este patrón.
CREATE VIEW IF NOT EXISTS v_tickets_by_status AS
SELECT Status, count() AS Tickets FROM fact_tickets GROUP BY Status;

CREATE VIEW IF NOT EXISTS v_tickets_by_priority AS
SELECT Priority, count() AS Tickets FROM fact_tickets GROUP BY Priority;

CREATE VIEW IF NOT EXISTS v_tickets_by_channel AS
SELECT if(Channel = '', 'sin_canal', Channel) AS Channel, count() AS Tickets FROM fact_tickets GROUP BY Channel;

CREATE VIEW IF NOT EXISTS v_tickets_by_category AS
SELECT if(Category = '', 'sin_categoria', Category) AS Category, count() AS Tickets FROM fact_tickets GROUP BY Category;

CREATE VIEW IF NOT EXISTS v_tickets_by_building AS
SELECT
    if(BuildingID = '', 'sin_edificio', BuildingID) AS BuildingID,
    count() AS Tickets,
    avgIf((toUnixTimestamp(ClosedAt) - toUnixTimestamp(CreatedAt)) / 3600, ClosedAt IS NOT NULL) AS AvgResolutionHours
FROM fact_tickets GROUP BY BuildingID;

CREATE VIEW IF NOT EXISTS v_mttr_by_priority AS
SELECT
    Priority,
    avgIf((toUnixTimestamp(ClosedAt) - toUnixTimestamp(CreatedAt)) / 3600, ClosedAt IS NOT NULL) AS MTTRHours
FROM fact_tickets GROUP BY Priority;

-- Semana = creados esa semana vs cerrados esa semana (independiente, mismo criterio que
-- weeklySeries() en api-go/internal/store/store.go — no "cerrados de lo creado esa semana").
CREATE VIEW IF NOT EXISTS v_weekly_trend AS
SELECT
    Week,
    sumIf(Cnt, Kind = 'created') AS Created,
    sumIf(Cnt, Kind = 'closed') AS Closed
FROM (
    SELECT toStartOfWeek(CreatedAt) AS Week, 'created' AS Kind, count() AS Cnt FROM fact_tickets GROUP BY Week
    UNION ALL
    SELECT toStartOfWeek(ClosedAt) AS Week, 'closed' AS Kind, count() AS Cnt FROM fact_tickets WHERE ClosedAt IS NOT NULL GROUP BY Week
)
GROUP BY Week ORDER BY Week;

CREATE VIEW IF NOT EXISTS v_csat AS
SELECT avg(SatisfactionScore) AS CSATAvg, count() AS RatedTickets FROM fact_tickets WHERE SatisfactionScore IS NOT NULL;

-- Agregado 2026-09-23 (más gráficos en Grafana, a pedido del usuario). v_aging_by_status es un
-- proxy simplificado: horas desde CreatedAt para tickets actualmente abiertos/en progreso/reabiertos,
-- agrupado por estado. Difiere del cálculo exacto de ReportsPage.tsx (que usa el último evento de
-- historial que llevó a ese estado, no siempre CreatedAt) — aceptable para un panel de Grafana,
-- documentado acá para no confundirlo con el número preciso que sale de /api/reports/summary.
CREATE VIEW IF NOT EXISTS v_aging_by_status AS
SELECT Status, avg((toUnixTimestamp(now()) - toUnixTimestamp(CreatedAt)) / 3600) AS AgingHours
FROM fact_tickets WHERE Status IN ('open', 'in_progress', 'reopened') GROUP BY Status;

CREATE VIEW IF NOT EXISTS v_csat_distribution AS
SELECT SatisfactionScore, count() AS Tickets FROM fact_tickets WHERE SatisfactionScore IS NOT NULL GROUP BY SatisfactionScore ORDER BY SatisfactionScore;

-- Tier hardcodeado por BuildingID: buildings vive solo en SQLite (no se sincroniza a ClickHouse),
-- y son 3 filas DEMO fijas (ver DOCUMENTACION.md §13) — mapear a mano acá es más simple que
-- sincronizar una tabla dim_buildings entera para 3 valores constantes.
CREATE VIEW IF NOT EXISTS v_tickets_by_tier AS
SELECT
    multiIf(BuildingID = 'bld_torre_norte', 'vip', BuildingID = '', 'sin_edificio', 'standard') AS Tier,
    count() AS Tickets
FROM fact_tickets GROUP BY Tier;

CREATE VIEW IF NOT EXISTS v_reopens_weekly AS
SELECT toStartOfWeek(Timestamp) AS Week, count() AS Reopens
FROM fact_events WHERE ToState = 'reopened' GROUP BY Week ORDER BY Week;

-- Valores crudos (no agregados) para el panel de histograma: el panel "histogram" de Grafana
-- arma los buckets él mismo del lado del cliente a partir de la lista de números.
CREATE VIEW IF NOT EXISTS v_resolution_hours AS
SELECT (toUnixTimestamp(ClosedAt) - toUnixTimestamp(CreatedAt)) / 3600 AS ResolutionHours
FROM fact_tickets WHERE ClosedAt IS NOT NULL;
