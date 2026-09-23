import React from 'react';
import { Link } from 'react-router-dom';
import { Ticket } from '../models/Ticket';
import { TicketStatusBadge, TicketPriorityTag } from './TicketStatusBadge';
import { formatRelative } from '../utils/date';

type Props = {
  ticket: Ticket;
  index?: number;
};

const initials = (name: string | null) => {
  if (!name) return '–';
  return name.split(' ').map((p) => p[0]).slice(0, 2).join('').toUpperCase();
};

const TicketRow: React.FC<Props> = ({ ticket, index = 0 }) => (
  <tr className="row-enter ticket-row" style={{ animationDelay: `${Math.min(index, 10) * 30}ms` }}>
    <td data-label="Ticket">
      <span className="ticket-id">{ticket.id}</span>
      <Link to={`/ticket/${ticket.id}`} className="ticket-title-link">{ticket.title}</Link>
    </td>
    <td data-label="Estado"><TicketStatusBadge status={ticket.status} /></td>
    <td data-label="Prioridad"><TicketPriorityTag priority={ticket.priority} /></td>
    <td data-label="Solicitante" style={{ fontSize: 12.5 }}>{ticket.requester}</td>
    <td data-label="Asignado">
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: ticket.assignedTo ? 'var(--ink)' : 'var(--ink-faint)' }}>
        <span className="avatar" style={{ width: 18, height: 18, fontSize: 9 }}>{initials(ticket.assignedTo)}</span>
        {ticket.assignedTo ?? 'Sin asignar'}
      </span>
    </td>
    <td data-label="Actualizado" className="muted">{formatRelative(ticket.updatedAt)}</td>
    <td className="ticket-row-go" style={{ textAlign: 'right' }}>
      <Link to={`/ticket/${ticket.id}`} aria-label="Ver ticket" style={{ color: 'var(--ink-faint)', display: 'inline-flex' }}>
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
          <polyline points="9 6 15 12 9 18" />
        </svg>
      </Link>
    </td>
  </tr>
);

export default TicketRow;
