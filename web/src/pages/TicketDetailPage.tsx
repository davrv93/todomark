import React, { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ticketService } from '../services/ticketService';
import { Ticket, TicketStatus } from '../models/Ticket';
import { TRANSITIONS } from '../models/transitions';
import { TicketStatusBadge, TicketPriorityTag, STATUS_META } from '../components/TicketStatusBadge';
import NotifyButton from '../components/NotifyButton';
import { formatDateTime, formatRelative } from '../utils/date';

const TicketDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [ticket, setTicket] = useState<Ticket | null>(null);
  const [error, setError] = useState('');
  const [transitioning, setTransitioning] = useState(false);

  const load = async () => {
    try {
      setTicket(await ticketService.getById(id ?? ''));
    } catch (e: any) {
      setError(e?.message ?? 'Error al cargar');
    }
  };

  useEffect(() => {
    load();
  }, [id]);

  if (error) return <p className="error-text">{error}</p>;
  if (!ticket) return <p className="muted">Cargando…</p>;

  const handleStatusChange = async (newStatus: TicketStatus) => {
    setError('');
    setTransitioning(true);
    try {
      setTicket(await ticketService.transition(ticket.id, newStatus, 'Usuario'));
    } catch (e: any) {
      setError(e?.message ?? 'Error en la transición');
    } finally {
      setTransitioning(false);
    }
  };

  const nextStatuses = TRANSITIONS[ticket.status] ?? [];

  return (
    <div className="detail-grid">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Link to="/" style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--ink-soft)', textDecoration: 'none', width: 'fit-content' }}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
          Volver a tickets
        </Link>

        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--ink-faint)' }}>{ticket.id}</span>
            <span style={{ width: 3, height: 3, borderRadius: '50%', background: 'var(--ink-faint)' }} />
            <span className="muted">Creado {formatDateTime(ticket.createdAt)} · Actualizado {formatRelative(ticket.updatedAt)}</span>
            <span style={{ flex: 1 }} />
            <Link to={`/ticket/${ticket.id}/edit`} className="btn btn-ghost" style={{ padding: '4px 8px', fontSize: 12.5 }}>Editar</Link>
          </div>
          <h1 style={{ margin: '6px 0 0', fontSize: 20, fontWeight: 700, letterSpacing: '-0.2px' }}>{ticket.title}</h1>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
          <TicketStatusBadge status={ticket.status} size="lg" />
          <TicketPriorityTag priority={ticket.priority} />
          <span style={{ flex: 1 }} />
          {nextStatuses.map((s) => (
            <button
              key={s}
              type="button"
              className="btn btn-outline-primary"
              disabled={transitioning}
              onClick={() => handleStatusChange(s)}
            >
              {STATUS_META[s].label}
            </button>
          ))}
        </div>

        <div className="card">
          <h2 style={{ fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.04em', color: 'var(--ink-faint)', marginBottom: 8 }}>Descripción</h2>
          <p style={{ margin: 0, fontSize: 13.5, lineHeight: 1.55 }}>{ticket.description}</p>
        </div>

        <div className="card">
          <h2 style={{ fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.04em', color: 'var(--ink-faint)', marginBottom: 12 }}>Detalles</h2>
          <div className="two-col-grid">
            <div>
              <div className="muted" style={{ marginBottom: 3 }}>Solicitante</div>
              <div style={{ fontSize: 13, fontWeight: 600 }}>{ticket.requester}</div>
            </div>
            <div>
              <div className="muted" style={{ marginBottom: 3 }}>Asignado a</div>
              <div style={{ fontSize: 13, fontWeight: 600 }}>{ticket.assignedTo ?? 'Sin asignar'}</div>
            </div>
            <div>
              <div className="muted" style={{ marginBottom: 3 }}>WhatsApp</div>
              <div style={{ fontSize: 13, fontWeight: 600 }}>{ticket.whatsappChatId ?? 'Sin asignar'}</div>
            </div>
          </div>
        </div>

        <NotifyButton ticket={ticket} />
      </div>

      <div className="card" style={{ display: 'flex', flexDirection: 'column' }}>
        <h2 style={{ fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.04em', color: 'var(--ink-faint)', marginBottom: 14 }}>Historial</h2>
        {ticket.history.length === 0 && <span className="muted">Sin eventos registrados.</span>}
        {ticket.history.map((e, i) => (
          <div key={i} style={{ display: 'flex', gap: 10, paddingBottom: 16 }}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', width: 10 }}>
              <span style={{ width: 9, height: 9, borderRadius: '50%', background: 'var(--primary)', flexShrink: 0, marginTop: 2 }} />
              {i < ticket.history.length - 1 && <span style={{ width: 1, flex: 1, background: 'var(--border)', marginTop: 4 }} />}
            </div>
            <div>
              <div style={{ fontSize: 12.5 }}><strong>{e.user}</strong> {e.action}</div>
              <div className="muted" style={{ marginTop: 2 }}>{formatDateTime(e.timestamp)}</div>
              {e.from && e.to && (
                <div style={{ fontSize: 11, color: 'var(--ink-soft)', marginTop: 4 }}>{e.from} → {e.to}</div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default TicketDetailPage;
