import React, { useEffect, useMemo, useState } from 'react';
import { Line } from 'react-chartjs-2';
import { CategoryScale, Chart as ChartJS, Legend, LinearScale, LineElement, PointElement, Tooltip } from 'chart.js';
import { ExecutiveSummary, ticketService } from '../services/ticketService';

ChartJS.register(CategoryScale, Legend, LinearScale, LineElement, PointElement, Tooltip);

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { labels: { boxWidth: 10, color: '#5B6472', font: { size: 11 } } } },
  scales: {
    x: { ticks: { color: '#5B6472', font: { size: 11 } }, grid: { color: '#E2E5EA' } },
    y: { ticks: { color: '#5B6472', font: { size: 11 }, precision: 0 }, grid: { color: '#E2E5EA' } },
  },
};

const deltaColor = (n: number) => (n > 0 ? 'var(--red-text)' : n < 0 ? 'var(--teal-text)' : 'var(--ink-faint)');
const deltaSign = (n: number) => (n > 0 ? `+${n}` : `${n}`);

const ExecutivePage: React.FC = () => {
  const [summary, setSummary] = useState<ExecutiveSummary | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    (async () => {
      try {
        setSummary(await ticketService.getExecutiveSummary());
      } catch (e: any) {
        setError(e?.message ?? 'Error al cargar el resumen ejecutivo');
      }
    })();
  }, []);

  const weeklyData = useMemo(() => {
    const buckets = summary?.weeklyTrend ?? [];
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

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div>
        <h1 style={{ margin: '0 0 4px', fontSize: 20, fontWeight: 700 }}>Vista gerencial</h1>
        <p className="muted" style={{ margin: 0 }}>
          Resumen de una pantalla. %SLA (umbrales demo), costo/hora ($25 demo) y edificios (3 demo)
          son datos ficticios de este proyecto demostrativo, no de un cliente real — ver `DOCUMENTACION.md` §13-14.
        </p>
      </div>

      <div className="two-col-grid">
        <div className="card">
          <span className="muted">Tickets abiertos ahora</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {summary.openNow}
          </div>
        </div>

        <div className="card">
          <span className="muted">Cambio neto (7 días)</span>
          <div
            style={{
              fontFamily: 'var(--font-display)',
              fontSize: 30,
              fontWeight: 700,
              marginTop: 6,
              color: deltaColor(summary.netChange7d),
            }}
          >
            {deltaSign(summary.netChange7d)}
          </div>
          <span className="muted" style={{ fontSize: 12 }}>creados − cerrados, últimos 7 días</span>
        </div>

        <div className="card">
          <span className="muted">MTTR (tiempo medio de resolución)</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {summary.mttrHours == null ? 'Sin datos' : `${summary.mttrHours.toFixed(1)} h`}
          </div>
        </div>

        <div className="card">
          <span className="muted">Tasa de reapertura</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {summary.reopenRatePct == null ? 'Sin datos' : `${summary.reopenRatePct.toFixed(1)}%`}
          </div>
        </div>

        <div className="card">
          <span className="muted">Satisfacción (CSAT)</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {summary.csatAvg == null ? 'Sin datos' : `${summary.csatAvg.toFixed(1)} / 5`}
          </div>
        </div>

        <div className="card">
          <span className="muted">% cumplimiento SLA (demo)</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {summary.slaCompliancePct == null ? 'Sin datos' : `${summary.slaCompliancePct.toFixed(1)}%`}
          </div>
        </div>

        <div className="card">
          <span className="muted">Costo estimado de soporte (demo, $25/h)</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {summary.costEstimateDemo == null ? 'Sin datos' : `$${summary.costEstimateDemo.toFixed(0)}`}
          </div>
        </div>

        <div className="card">
          <span className="muted">Embudo: visitas → leads</span>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, fontWeight: 700, marginTop: 6 }}>
            {summary.landingVisits} → {summary.leadsTotal}
          </div>
          <span className="muted" style={{ fontSize: 12 }}>
            {summary.landingVisits === 0 ? 'Trackeo de visitas recién empezó' : `${((summary.leadsTotal / summary.landingVisits) * 100).toFixed(1)}% conversión`}
          </span>
        </div>
      </div>

      <section className="card" style={{ minHeight: 260 }}>
        <h2 style={{ fontSize: 14, marginBottom: 14 }}>Tendencia semanal (creados vs cerrados)</h2>
        <div style={{ height: 200 }}><Line data={weeklyData} options={chartOptions} /></div>
      </section>
    </div>
  );
};

export default ExecutivePage;
