import React, { useEffect, useRef, useState } from 'react';
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
import type { ReportChatChart, ReportChatReply, ReportChatTable } from '../services/ticketService';
import { ticketService } from '../services/ticketService';

ChartJS.register(ArcElement, BarElement, CategoryScale, Legend, LinearScale, LineElement, PointElement, Tooltip);

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

type Provider = 'gemini' | 'deepseek';
type Msg = { role: 'user' | 'assistant'; reply: string; table?: ReportChatTable | null; chart?: ReportChatChart | null; error?: boolean };

const ChartBlock: React.FC<{ chart: ReportChatChart }> = ({ chart }) => {
  const data = {
    labels: chart.labels,
    datasets: chart.datasets.map((d, i) => ({
      label: d.label,
      data: d.data,
      backgroundColor: chart.type === 'line' ? palette[i % palette.length] + '33' : palette,
      borderColor: chart.type === 'line' ? palette[i % palette.length] : palette,
      borderWidth: 1,
      tension: 0.25,
    })),
  };
  const opts = chart.type === 'doughnut' ? { ...chartOptions, scales: undefined } : chartOptions;
  return (
    <div style={{ height: 220, marginTop: 10 }}>
      {chart.type === 'bar' && <Bar data={data} options={opts} />}
      {chart.type === 'line' && <Line data={data} options={opts} />}
      {chart.type === 'doughnut' && <Doughnut data={data} options={opts} />}
    </div>
  );
};

const TableBlock: React.FC<{ table: ReportChatTable }> = ({ table }) => (
  <div style={{ overflowX: 'auto', marginTop: 10 }}>
    <table className="ticket-table">
      <thead><tr>{table.columns.map((c) => <th key={c}>{c}</th>)}</tr></thead>
      <tbody>
        {table.rows.map((row, i) => (
          <tr key={i}>{row.map((cell, j) => <td key={j}>{cell}</td>)}</tr>
        ))}
      </tbody>
    </table>
  </div>
);

const ReportChatPage: React.FC = () => {
  const [provider, setProvider] = useState<Provider>('gemini');
  const [input, setInput] = useState('');
  const [messages, setMessages] = useState<Msg[]>([]);
  const [loading, setLoading] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const send = async (e: React.FormEvent) => {
    e.preventDefault();
    const question = input.trim();
    if (!question || loading) return;
    setInput('');
    setMessages((m) => [...m, { role: 'user', reply: question }]);
    setLoading(true);
    try {
      const res: ReportChatReply = await ticketService.reportChat(question, provider);
      setMessages((m) => [...m, { role: 'assistant', reply: res.reply, table: res.table, chart: res.chart }]);
    } catch (err: any) {
      setMessages((m) => [...m, { role: 'assistant', reply: err?.message ?? 'No se pudo contactar al modelo.', error: true }]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
        <div>
          <h1 style={{ margin: '0 0 4px', fontSize: 20, fontWeight: 700 }}>Chat de reportería</h1>
          <p className="muted" style={{ margin: 0 }}>
            Preguntá en lenguaje natural. Usa datos reales de <code>/reports</code> y <code>/executive</code> —
            sin embeddings todavía (Fase 23 propuesta, ver <code>PLAN.md</code>).
          </p>
        </div>
        <div className="field" style={{ margin: 0 }}>
          <label htmlFor="rc-provider" style={{ fontSize: 11 }}>Modelo</label>
          <select id="rc-provider" className="select" value={provider} onChange={(e) => setProvider(e.target.value as Provider)}>
            <option value="gemini">Gemini</option>
            <option value="deepseek">DeepSeek</option>
          </select>
        </div>
      </div>

      <div className="card" style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 14, minHeight: 380, maxHeight: '60vh', overflowY: 'auto' }}>
        {messages.length === 0 && (
          <p className="muted">
            Ej: "¿cuántos tickets abrimos esta semana?", "dame los tickets por edificio", "¿cómo viene el MTTR por prioridad?"
          </p>
        )}
        {messages.map((m, i) => (
          <div key={i} style={{ alignSelf: m.role === 'user' ? 'flex-end' : 'flex-start', maxWidth: '85%' }}>
            <div
              className={m.role === 'user' ? 'badge' : undefined}
              style={{
                background: m.role === 'user' ? 'var(--primary-tint)' : 'var(--surface-alt)',
                color: m.error ? 'var(--red-text)' : m.role === 'user' ? 'var(--primary-dark)' : 'var(--ink)',
                borderRadius: 'var(--radius-sm)',
                padding: '10px 12px',
                fontSize: 13.5,
                lineHeight: 1.5,
              }}
            >
              {m.reply}
              {m.table && <TableBlock table={m.table} />}
              {m.chart && <ChartBlock chart={m.chart} />}
            </div>
          </div>
        ))}
        {loading && <p className="muted">Pensando…</p>}
        <div ref={bottomRef} />
      </div>

      <form onSubmit={send} style={{ display: 'flex', gap: 8 }}>
        <input
          className="input"
          style={{ flex: 1 }}
          placeholder="Preguntá algo sobre los tickets, edificios, agentes..."
          value={input}
          onChange={(e) => setInput(e.target.value)}
        />
        <button type="submit" className="btn btn-primary" disabled={loading || !input.trim()}>Enviar</button>
      </form>
    </div>
  );
};

export default ReportChatPage;
