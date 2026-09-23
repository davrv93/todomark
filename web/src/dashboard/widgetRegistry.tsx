import React from 'react';
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
import type { ExecutiveSummary, ReportSummary } from '../services/ticketService';

ChartJS.register(ArcElement, BarElement, CategoryScale, Legend, LinearScale, LineElement, PointElement, Tooltip);

export type DashboardData = { report: ReportSummary; exec: ExecutiveSummary };
export type WidgetSize = 'sm' | 'md' | 'lg';
export type WidgetDef = { id: string; title: string; size: WidgetSize; render: (data: DashboardData) => React.ReactNode };

export const SIZE_SPAN: Record<WidgetSize, number> = { sm: 3, md: 6, lg: 12 };

const palette = ['#1E4FD8', '#C77B12', '#0E8C7F', '#6B7280', '#C23B3B', '#7C3AED'];
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

const Kpi: React.FC<{ label: string; value: React.ReactNode; sub?: string; color?: string }> = ({ label, value, sub, color }) => (
  <div style={{ height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
    <span className="muted">{label}</span>
    <div style={{ fontFamily: 'var(--font-display)', fontSize: 28, fontWeight: 700, marginTop: 6, color }}>{value}</div>
    {sub && <span className="muted" style={{ fontSize: 12 }}>{sub}</span>}
  </div>
);

const fmtH = (n: number | null) => (n == null ? 'Sin datos' : `${n.toFixed(1)} h`);

export const WIDGETS: WidgetDef[] = [
  { id: 'kpi-open', title: 'Tickets abiertos', size: 'sm', render: ({ exec }) => <Kpi label="Tickets abiertos" value={exec.openNow} /> },
  { id: 'kpi-net', title: 'Cambio neto (7 días)', size: 'sm', render: ({ exec }) => (
    <Kpi label="Cambio neto (7 días)" value={exec.netChange7d > 0 ? `+${exec.netChange7d}` : exec.netChange7d}
      color={exec.netChange7d > 0 ? 'var(--red-text)' : exec.netChange7d < 0 ? 'var(--teal-text)' : undefined}
      sub="creados − cerrados" />
  ) },
  { id: 'kpi-mttr', title: 'MTTR global', size: 'sm', render: ({ exec }) => <Kpi label="MTTR" value={fmtH(exec.mttrHours)} /> },
  { id: 'kpi-csat', title: 'CSAT promedio', size: 'sm', render: ({ exec }) => <Kpi label="Satisfacción (CSAT)" value={exec.csatAvg == null ? 'Sin datos' : `${exec.csatAvg.toFixed(1)} / 5`} /> },
  { id: 'kpi-sla', title: '% cumplimiento SLA (demo)', size: 'sm', render: ({ exec }) => <Kpi label="% cumplimiento SLA (demo)" value={exec.slaCompliancePct == null ? 'Sin datos' : `${exec.slaCompliancePct.toFixed(1)}%`} /> },
  { id: 'kpi-reopen', title: 'Tasa de reapertura', size: 'sm', render: ({ exec }) => <Kpi label="Tasa de reapertura" value={exec.reopenRatePct == null ? 'Sin datos' : `${exec.reopenRatePct.toFixed(1)}%`} /> },
  { id: 'kpi-cost', title: 'Costo estimado (demo)', size: 'sm', render: ({ exec }) => <Kpi label="Costo estimado (demo, $25/h)" value={exec.costEstimateDemo == null ? 'Sin datos' : `$${exec.costEstimateDemo.toFixed(0)}`} /> },
  { id: 'kpi-unassigned', title: 'Sin asignar', size: 'sm', render: ({ report }) => <Kpi label="Sin asignar" value={report.unassignedCount} sub={report.unassignedOldestHours == null ? undefined : `más antiguo: ${report.unassignedOldestHours.toFixed(0)}h`} /> },
  { id: 'kpi-sla-risk', title: 'SLA en riesgo/vencido (demo)', size: 'sm', render: ({ report }) => <Kpi label="SLA en riesgo / vencido" value={`${report.slaAtRiskCount} / ${report.slaOverdueCount}`} color={report.slaOverdueCount > 0 ? 'var(--red-text)' : undefined} /> },
  { id: 'kpi-frt', title: 'Primera respuesta (FRT)', size: 'sm', render: ({ report }) => <Kpi label="Primera respuesta (FRT)" value={fmtH(report.frtHours)} /> },

  { id: 'chart-status', title: 'Por estado', size: 'md', render: ({ report }) => (
    <div style={{ height: 220 }}><Doughnut data={makeData('Estado', report.byStatus)} options={{ ...chartOptions, scales: undefined }} /></div>
  ) },
  { id: 'chart-priority', title: 'Por prioridad', size: 'md', render: ({ report }) => (
    <div style={{ height: 220 }}><Bar data={makeData('Prioridad', report.byPriority)} options={chartOptions} /></div>
  ) },
  { id: 'chart-channel', title: 'Por canal', size: 'md', render: ({ report }) => (
    <div style={{ height: 220 }}><Doughnut data={makeData('Canal', report.byChannel)} options={{ ...chartOptions, scales: undefined }} /></div>
  ) },
  { id: 'chart-category', title: 'Por categoría', size: 'md', render: ({ report }) => (
    <div style={{ height: 220 }}><Doughnut data={makeData('Categoría', report.byCategory)} options={{ ...chartOptions, scales: undefined }} /></div>
  ) },
  { id: 'chart-agent', title: 'Carga por agente', size: 'md', render: ({ report }) => (
    <div style={{ height: 220 }}><Bar data={makeData('Tickets', report.byAgent)} options={chartOptions} /></div>
  ) },
  { id: 'chart-weekly', title: 'Tendencia semanal', size: 'lg', render: ({ exec }) => {
    const buckets = exec.weeklyTrend ?? [];
    const data = {
      labels: buckets.map((b) => b.weekStart),
      datasets: [
        { label: 'Creados', data: buckets.map((b) => b.created), borderColor: '#1E4FD8', backgroundColor: '#EAF0FE', tension: 0.25 },
        { label: 'Cerrados', data: buckets.map((b) => b.closed), borderColor: '#0E8C7F', backgroundColor: '#E3F5F2', tension: 0.25 },
      ],
    };
    return <div style={{ height: 240 }}><Line data={data} options={chartOptions} /></div>;
  } },
  { id: 'table-buildings', title: 'Top edificios (demo)', size: 'lg', render: ({ report }) => (
    <div style={{ overflowX: 'auto' }}>
      <table className="ticket-table">
        <thead><tr><th>Edificio</th><th>Tier</th><th>Tickets</th><th>MTTR</th></tr></thead>
        <tbody>
          {report.byBuilding.map((b) => (
            <tr key={b.buildingId}>
              <td>{b.name}</td><td>{b.tier || '—'}</td><td>{b.count}</td>
              <td>{b.avgResolutionHours == null ? '—' : `${b.avgResolutionHours.toFixed(1)} h`}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  ) },
];

export const DEFAULT_LAYOUT = ['kpi-open', 'kpi-mttr', 'kpi-csat', 'kpi-sla', 'chart-status', 'chart-priority', 'chart-weekly'];

export const widgetById = (id: string): WidgetDef | undefined => WIDGETS.find((w) => w.id === id);
