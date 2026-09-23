import React, { useEffect, useMemo, useState } from 'react';
import { Bar, Doughnut } from 'react-chartjs-2';
import {
  ArcElement,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  Tooltip,
} from 'chart.js';
import type { Ticket } from '../models/Ticket';
import type { Building, BuildingStat } from '../services/ticketService';
import { ticketService } from '../services/ticketService';
import { formatRelative } from '../utils/date';
import LiveBadge from './LiveBadge';
import { TicketPriorityTag, TicketStatusBadge } from './TicketStatusBadge';

ChartJS.register(ArcElement, BarElement, CategoryScale, Legend, LinearScale, Tooltip);

const palette = ['#1E4FD8', '#C77B12', '#0E8C7F', '#6B7280', '#C23B3B'];

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { labels: { boxWidth: 10, color: '#5B6472', font: { size: 11 } } } },
  scales: {
    x: { ticks: { color: '#5B6472', font: { size: 11 } }, grid: { color: '#E2E5EA' } },
    y: { ticks: { color: '#5B6472', font: { size: 11 }, precision: 0 }, grid: { color: '#E2E5EA' } },
  },
};

const makeData = (label: string, values: Record<string, number>) => ({
  labels: Object.keys(values),
  datasets: [{ label, data: Object.values(values), backgroundColor: palette, borderColor: palette, borderWidth: 1 }],
});

function groupBy(tickets: Ticket[], key: 'status' | 'priority'): Record<string, number> {
  const out: Record<string, number> = {};
  for (const t of tickets) out[t[key]] = (out[t[key]] ?? 0) + 1;
  return out;
}

const StarRating: React.FC<{ ticket: Ticket; onRated: (updated: Ticket) => void }> = ({ ticket, onRated }) => {
  const [saving, setSaving] = useState(false);

  if (ticket.satisfactionScore != null) {
    return (
      <span className="muted" style={{ fontSize: 12 }}>
        Tu calificación: {'★'.repeat(ticket.satisfactionScore)}{'☆'.repeat(5 - ticket.satisfactionScore)}
      </span>
    );
  }
  if (ticket.status !== 'resolved' && ticket.status !== 'closed') return null;

  const rate = async (score: number) => {
    setSaving(true);
    try {
      onRated(await ticketService.setSatisfaction(ticket.id, score));
    } catch {
      setSaving(false);
    }
  };

  return (
    <span style={{ display: 'inline-flex', gap: 2 }}>
      {[1, 2, 3, 4, 5].map((n) => (
        <button
          key={n}
          type="button"
          disabled={saving}
          onClick={() => rate(n)}
          title={`Calificar con ${n} estrella${n > 1 ? 's' : ''}`}
          style={{ background: 'none', border: 'none', cursor: saving ? 'default' : 'pointer', padding: 0, fontSize: 16, color: 'var(--amber)', lineHeight: 1 }}
        >
          ☆
        </button>
      ))}
    </span>
  );
};

const EscalateButton: React.FC<{ ticket: Ticket; onEscalated: (updated: Ticket) => void }> = ({ ticket, onEscalated }) => {
  const [saving, setSaving] = useState(false);
  const escalated = ticket.history.some((e) => e.action === 'escalated');

  if (ticket.status !== 'open' && ticket.status !== 'in_progress' && ticket.status !== 'reopened') return null;
  if (escalated) return <span className="muted" style={{ fontSize: 12 }}>Escalado ✓</span>;

  const escalate = async () => {
    setSaving(true);
    try {
      onEscalated(await ticketService.escalate(ticket.id));
    } catch {
      setSaving(false);
    }
  };

  return (
    <button type="button" className="btn" disabled={saving} onClick={escalate} style={{ fontSize: 12, padding: '4px 10px' }}>
      {saving ? 'Escalando…' : 'Escalar a gerente'}
    </button>
  );
};

function averageResolutionHours(tickets: Ticket[]): number | null {
  const resolved = tickets.filter((t) => t.closedAt);
  if (resolved.length === 0) return null;
  const totalHours = resolved.reduce((sum, t) => {
    const created = new Date(t.createdAt).getTime();
    const closed = new Date(t.closedAt as string).getTime();
    return sum + (closed - created) / 3_600_000;
  }, 0);
  return totalHours / resolved.length;
}

const ClientReportsIsland: React.FC = () => {
  const [email, setEmail] = useState('');
  const [tickets, setTickets] = useState<Ticket[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [buildings, setBuildings] = useState<Building[]>([]);
  const [buildingStats, setBuildingStats] = useState<BuildingStat[]>([]);

  // U2/U3: nombre de edificio y MTTR comparativo del edificio (datos demo, ver DOCUMENTACION.md §13).
  useEffect(() => {
    ticketService.getBuildings().then(setBuildings).catch(() => {});
    ticketService.getReportSummary().then((r) => setBuildingStats(r.byBuilding)).catch(() => {});
  }, []);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim()) return;
    setLoading(true);
    setError('');
    try {
      setTickets(await ticketService.getByEmail(email.trim()));
    } catch (err: any) {
      setError(err?.message ?? 'No se pudo cargar tus tickets');
      setTickets(null);
    } finally {
      setLoading(false);
    }
  };

  const byStatus = useMemo(() => groupBy(tickets ?? [], 'status'), [tickets]);
  const byPriority = useMemo(() => groupBy(tickets ?? [], 'priority'), [tickets]);
  const avgHours = useMemo(() => averageResolutionHours(tickets ?? []), [tickets]);
  const openCount = (byStatus.open ?? 0) + (byStatus.in_progress ?? 0) + (byStatus.reopened ?? 0);
  const resolvedCount = (byStatus.resolved ?? 0) + (byStatus.closed ?? 0);
  const recent = useMemo(
    () => [...(tickets ?? [])].sort((a, b) => +new Date(b.updatedAt) - +new Date(a.updatedAt)).slice(0, 8),
    [tickets],
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4 }}>
          <h1 style={{ margin: 0, fontSize: 20, fontWeight: 700 }}>Mis reportes</h1>
          <LiveBadge />
        </div>
        <p className="muted" style={{ margin: 0 }}>Ingresa tu correo para ver el estado de tus tickets de soporte.</p>
      </div>

      <form onSubmit={onSubmit} className="card" style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
        <input
          type="email"
          required
          placeholder="tu@empresa.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          style={{
            flex: '1 1 240px',
            padding: '10px 12px',
            borderRadius: 8,
            border: '1px solid var(--border)',
            font: 'inherit',
            fontSize: 13,
          }}
        />
        <button type="submit" className="btn btn-primary" disabled={loading}>
          {loading ? 'Buscando…' : 'Ver mis reportes'}
        </button>
      </form>

      {error && <p className="error-text">{error}</p>}

      {tickets && tickets.length === 0 && !error && (
        <p className="muted">No encontramos tickets asociados a ese correo.</p>
      )}

      {tickets && tickets.length > 0 && (
        <>
          <div className="two-col-grid">
            <div className="card">
              <span className="muted">Tickets totales</span>
              <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>{tickets.length}</div>
            </div>
            <div className="card">
              <span className="muted">Abiertos / en progreso</span>
              <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>{openCount}</div>
            </div>
            <div className="card">
              <span className="muted">Resueltos / cerrados</span>
              <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>{resolvedCount}</div>
            </div>
            <div className="card">
              <span className="muted">Tiempo medio de resolución</span>
              <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
                {avgHours == null ? 'Sin datos' : `${avgHours.toFixed(1)} h`}
              </div>
            </div>
          </div>

          <div className="two-col-grid">
            <section className="card" style={{ minHeight: 260 }}>
              <h2 style={{ fontSize: 14, marginBottom: 14 }}>Por estado</h2>
              <div style={{ height: 200 }}><Doughnut data={makeData('Estado', byStatus)} options={{ ...chartOptions, scales: undefined }} /></div>
            </section>
            <section className="card" style={{ minHeight: 260 }}>
              <h2 style={{ fontSize: 14, marginBottom: 14 }}>Por prioridad</h2>
              <div style={{ height: 200 }}><Bar data={makeData('Prioridad', byPriority)} options={chartOptions} /></div>
            </section>
          </div>

          <section className="card">
            <h2 style={{ fontSize: 14, marginBottom: 14 }}>Últimos tickets</h2>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {recent.map((t) => (
                <div
                  key={t.id}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 12,
                    padding: '10px 0',
                    borderBottom: '1px solid var(--border)',
                    flexWrap: 'wrap',
                  }}
                >
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontWeight: 600, fontSize: 13 }}>{t.title}</div>
                    <div className="muted" style={{ fontSize: 12 }}>Actualizado {formatRelative(t.updatedAt)}</div>
                    {t.buildingId && (() => {
                      const b = buildings.find((x) => x.id === t.buildingId);
                      const stat = buildingStats.find((x) => x.buildingId === t.buildingId);
                      return (
                        <div className="muted" style={{ fontSize: 12 }}>
                          {b?.name ?? t.buildingId}{t.unitId ? ` · unidad ${t.unitId}` : ''}
                          {stat?.avgResolutionHours != null ? ` · MTTR del edificio: ${stat.avgResolutionHours.toFixed(1)} h` : ''}
                        </div>
                      );
                    })()}
                  </div>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <TicketPriorityTag priority={t.priority} />
                    <TicketStatusBadge status={t.status} />
                    <StarRating
                      ticket={t}
                      onRated={(updated) => setTickets((prev) => (prev ?? []).map((x) => (x.id === updated.id ? updated : x)))}
                    />
                    <EscalateButton
                      ticket={t}
                      onEscalated={(updated) => setTickets((prev) => (prev ?? []).map((x) => (x.id === updated.id ? updated : x)))}
                    />
                  </div>
                </div>
              ))}
            </div>
          </section>
        </>
      )}
    </div>
  );
};

export default ClientReportsIsland;
