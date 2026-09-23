import React, { useEffect, useMemo, useState } from 'react';
import { Bar, Doughnut, Line } from 'react-chartjs-2';
import {
  ArcElement,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip,
} from 'chart.js';
import { ReportSummary, ticketService } from '../services/ticketService';

ChartJS.register(ArcElement, BarElement, CategoryScale, Legend, LinearScale, LineElement, PointElement, Tooltip);

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

const ReportsPage: React.FC = () => {
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    (async () => {
      try {
        setSummary(await ticketService.getReportSummary());
      } catch (e: any) {
        setError(e?.message ?? 'Error al cargar reportes');
      }
    })();
  }, []);

  const dailyData = useMemo(() => {
    const values = summary?.dailyTickets ?? {};
    return {
      labels: Object.keys(values),
      datasets: [{
        label: 'Tickets diarios',
        data: Object.values(values),
        borderColor: '#1E4FD8',
        backgroundColor: '#EAF0FE',
        tension: 0.25,
      }],
    };
  }, [summary]);

  const weeklyData = useMemo(() => {
    const buckets = summary?.weeklyBacklog ?? [];
    return {
      labels: buckets.map((b) => b.weekStart),
      datasets: [
        { label: 'Creados', data: buckets.map((b) => b.created), borderColor: '#1E4FD8', backgroundColor: '#EAF0FE', tension: 0.25 },
        { label: 'Cerrados', data: buckets.map((b) => b.closed), borderColor: '#0E8C7F', backgroundColor: '#E3F5F2', tension: 0.25 },
      ],
    };
  }, [summary]);

  if (error) return <p className="error-text">{error}</p>;
  if (!summary) return <p className="muted">Cargando…</p>;

  const total = Object.values(summary.byStatus).reduce((sum, value) => sum + value, 0);
  const avg = summary.averageResolutionHours;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div>
        <h1 style={{ margin: '0 0 4px', fontSize: 20, fontWeight: 700 }}>Reportes</h1>
        <p className="muted" style={{ margin: 0 }}>Resumen operativo de tickets por estado, prioridad y fecha.</p>
      </div>

      <div className="two-col-grid">
        <div className="card">
          <span className="muted">Tickets totales</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>{total}</div>
        </div>
        <div className="card">
          <span className="muted">Tiempo medio de resolución</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {avg == null ? 'Sin datos' : `${avg.toFixed(1)} h`}
          </div>
        </div>
        <div className="card">
          <span className="muted">Sin asignar</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>{summary.unassignedCount}</div>
          <span className="muted" style={{ fontSize: 12 }}>
            {summary.unassignedOldestHours == null ? '' : `más antiguo: ${summary.unassignedOldestHours.toFixed(0)}h`}
          </span>
        </div>
        <div className="card">
          <span className="muted">Reaperturas hoy</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>{summary.reopensToday}</div>
        </div>
        <div className="card">
          <span className="muted">SLA en riesgo (demo, &lt;2h)</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>{summary.slaAtRiskCount}</div>
        </div>
        <div className="card">
          <span className="muted">SLA vencido (demo)</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6, color: summary.slaOverdueCount > 0 ? 'var(--red-text)' : undefined }}>
            {summary.slaOverdueCount}
          </div>
        </div>
        <div className="card">
          <span className="muted">Primera respuesta (FRT)</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {summary.frtHours == null ? 'Sin datos' : `${summary.frtHours.toFixed(1)} h`}
          </div>
        </div>
      </div>

      <div className="two-col-grid">
        <section className="card" style={{ minHeight: 300 }}>
          <h2 style={{ fontSize: 14, marginBottom: 14 }}>Por estado</h2>
          <div style={{ height: 230 }}><Doughnut data={makeData('Estado', summary.byStatus)} options={{ ...chartOptions, scales: undefined }} /></div>
        </section>
        <section className="card" style={{ minHeight: 300 }}>
          <h2 style={{ fontSize: 14, marginBottom: 14 }}>Por prioridad</h2>
          <div style={{ height: 230 }}><Bar data={makeData('Prioridad', summary.byPriority)} options={chartOptions} /></div>
        </section>
      </div>

      <section className="card" style={{ minHeight: 300 }}>
        <h2 style={{ fontSize: 14, marginBottom: 14 }}>Tickets diarios</h2>
        <div style={{ height: 230 }}><Line data={dailyData} options={chartOptions} /></div>
      </section>

      <section className="card" style={{ minHeight: 300 }}>
        <h2 style={{ fontSize: 14, marginBottom: 14 }}>Carga por agente</h2>
        <p className="muted" style={{ fontSize: 12, marginTop: -8, marginBottom: 14 }}>
          Solo tickets abiertos/en progreso/reabiertos (WIP). "sin_asignar" = sin agente asignado.
        </p>
        <div style={{ height: 230 }}><Bar data={makeData('Tickets', summary.byAgent)} options={chartOptions} /></div>
      </section>

      <section className="card" style={{ minHeight: 300 }}>
        <h2 style={{ fontSize: 14, marginBottom: 14 }}>Por canal de entrada</h2>
        <p className="muted" style={{ fontSize: 12, marginTop: -8, marginBottom: 14 }}>
          "sin_canal" son tickets creados antes de rastrear el canal.
        </p>
        <div style={{ height: 230 }}><Doughnut data={makeData('Canal', summary.byChannel)} options={{ ...chartOptions, scales: undefined }} /></div>
      </section>

      <section className="card" style={{ minHeight: 300 }}>
        <h2 style={{ fontSize: 14, marginBottom: 14 }}>Por categoría</h2>
        <p className="muted" style={{ fontSize: 12, marginTop: -8, marginBottom: 14 }}>
          "sin_categoria" son tickets sin categoría elegida al crearse.
        </p>
        <div style={{ height: 230 }}><Doughnut data={makeData('Categoría', summary.byCategory)} options={{ ...chartOptions, scales: undefined }} /></div>
      </section>

      <section className="card" style={{ minHeight: 300 }}>
        <h2 style={{ fontSize: 14, marginBottom: 14 }}>Aging por estado (horas desde que entró a ese estado)</h2>
        <div style={{ height: 230 }}><Bar data={makeData('Horas', summary.agingHoursByStatus)} options={chartOptions} /></div>
      </section>

      <section className="card" style={{ minHeight: 300 }}>
        <h2 style={{ fontSize: 14, marginBottom: 14 }}>Backlog semanal (creados vs cerrados)</h2>
        <div style={{ height: 230 }}><Line data={weeklyData} options={chartOptions} /></div>
      </section>

      <section className="card">
        <h2 style={{ fontSize: 14, marginBottom: 4 }}>Top edificios (demo)</h2>
        <p className="muted" style={{ fontSize: 12, marginBottom: 14 }}>
          3 edificios ficticios sembrados para esta demo — no son datos de un cliente real.
        </p>
        <div style={{ overflowX: 'auto' }}>
          <table className="ticket-table">
            <thead>
              <tr><th>Edificio</th><th>Tier</th><th>Tickets</th><th>MTTR</th></tr>
            </thead>
            <tbody>
              {summary.byBuilding.map((b) => (
                <tr key={b.buildingId}>
                  <td>{b.name}</td>
                  <td>{b.tier || '—'}</td>
                  <td>{b.count}</td>
                  <td>{b.avgResolutionHours == null ? '—' : `${b.avgResolutionHours.toFixed(1)} h`}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card">
        <h2 style={{ fontSize: 14, marginBottom: 4 }}>Edificio × categoría (demo)</h2>
        <p className="muted" style={{ fontSize: 12, marginBottom: 14 }}>Cuenta de tickets por combinación.</p>
        <div style={{ overflowX: 'auto' }}>
          <table className="ticket-table">
            <thead>
              <tr><th>Edificio</th><th>Categoría</th><th>Tickets</th></tr>
            </thead>
            <tbody>
              {summary.categoryByBuilding.map((cb, i) => (
                <tr key={i}>
                  <td>{cb.buildingName}</td>
                  <td>{cb.category}</td>
                  <td>{cb.count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
};

export default ReportsPage;
