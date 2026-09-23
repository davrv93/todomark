import React from 'react';
import type { TicketStatus, Priority } from '../models/Ticket';

type StatusMeta = { label: string; bg: string; color: string; dot: string };

export const STATUS_META: Record<TicketStatus, StatusMeta> = {
  open: { label: 'Abierto', bg: 'var(--primary-tint)', color: 'var(--primary-dark)', dot: 'var(--primary)' },
  in_progress: { label: 'En progreso', bg: 'var(--amber-tint)', color: 'var(--amber-text)', dot: 'var(--amber)' },
  resolved: { label: 'Resuelto', bg: 'var(--teal-tint)', color: 'var(--teal-text)', dot: 'var(--teal)' },
  closed: { label: 'Cerrado', bg: 'var(--gray-tint)', color: 'var(--gray-text)', dot: 'var(--gray)' },
  reopened: { label: 'Reabierto', bg: 'var(--red-tint)', color: 'var(--red-text)', dot: 'var(--red)' },
};

export const PRIORITY_META: Record<Priority, { label: string; dot: string }> = {
  low: { label: 'Baja', dot: 'var(--ink-faint)' },
  medium: { label: 'Media', dot: 'var(--primary)' },
  high: { label: 'Alta', dot: 'var(--amber)' },
  critical: { label: 'Crítica', dot: 'var(--red)' },
};

type BadgeProps = { status: TicketStatus; size?: 'sm' | 'lg' };

export const TicketStatusBadge: React.FC<BadgeProps> = ({ status, size = 'sm' }) => {
  const m = STATUS_META[status];
  return (
    <span className={`badge${size === 'lg' ? ' badge-lg' : ''}`} style={{ background: m.bg, color: m.color }}>
      <span className="badge-dot" style={{ background: m.dot }} />
      {m.label}
    </span>
  );
};

export const TicketPriorityTag: React.FC<{ priority: Priority }> = ({ priority }) => {
  const m = PRIORITY_META[priority];
  return (
    <span className="priority-tag">
      <span className="badge-dot" style={{ background: m.dot }} />
      {m.label}
    </span>
  );
};

export default TicketStatusBadge;
