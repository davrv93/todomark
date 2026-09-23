import React, { useEffect, useMemo, useState } from 'react';
import { DndContext, PointerSensor, useDroppable, useSensor, useSensors } from '@dnd-kit/core';
import type { DragEndEvent } from '@dnd-kit/core';
import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Ticket, TicketStatus } from '../models/Ticket';
import { TRANSITIONS } from '../models/transitions';
import { ticketService } from '../services/ticketService';
import { TicketPriorityTag, TicketStatusBadge } from './TicketStatusBadge';
import { formatRelative } from '../utils/date';

const COLUMNS: TicketStatus[] = ['open', 'in_progress', 'resolved', 'closed'];

const Column: React.FC<{ status: TicketStatus; tickets: Ticket[] }> = ({ status, tickets }) => {
  const { setNodeRef, isOver } = useDroppable({ id: status });

  return (
    <section className="card" style={{ padding: 12, minHeight: 280, display: 'flex', flexDirection: 'column', gap: 10, background: isOver ? 'var(--primary-tint)' : 'var(--surface)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8 }}>
        <TicketStatusBadge status={status} />
        <span className="muted">{tickets.length}</span>
      </div>
      <SortableContext items={tickets.map((t) => t.id)} strategy={verticalListSortingStrategy}>
        <div ref={setNodeRef} style={{ display: 'flex', flexDirection: 'column', gap: 8, flex: 1 }}>
          {tickets.map((ticket) => <KanbanCard key={ticket.id} ticket={ticket} />)}
          {tickets.length === 0 && <div className="muted" style={{ padding: '18px 6px', textAlign: 'center' }}>Sin tickets</div>}
        </div>
      </SortableContext>
    </section>
  );
};

const KanbanCard: React.FC<{ ticket: Ticket }> = ({ ticket }) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: ticket.id });
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.6 : 1,
    cursor: 'grab',
  };

  return (
    <a
      ref={setNodeRef}
      href={`/ticket/${ticket.id}`}
      style={{ ...style, display: 'flex', flexDirection: 'column', gap: 8, padding: 10, border: '1px solid var(--border)', borderRadius: 'var(--radius-sm)', background: 'var(--surface)', textDecoration: 'none' }}
      {...attributes}
      {...listeners}
    >
      <span className="ticket-id">{ticket.id}</span>
      <strong style={{ fontSize: 13.5, lineHeight: 1.3 }}>{ticket.title}</strong>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
        <TicketPriorityTag priority={ticket.priority} />
        <span className="muted">{formatRelative(ticket.updatedAt)}</span>
      </div>
      <span className="muted">{ticket.assignedTeam || ticket.assignedTo || 'Sin asignar'}</span>
    </a>
  );
};

const KanbanIsland: React.FC = () => {
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      setTickets(await ticketService.getAll());
    } catch (e: any) {
      setError(e?.message ?? 'Error al cargar Kanban');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const grouped = useMemo(() => {
    return COLUMNS.reduce<Record<TicketStatus, Ticket[]>>((acc, status) => {
      acc[status] = tickets.filter((t) => t.status === status);
      return acc;
    }, { open: [], in_progress: [], resolved: [], closed: [], reopened: [] });
  }, [tickets]);

  const onDragEnd = async ({ active, over }: DragEndEvent) => {
    if (!over) return;
    const ticket = tickets.find((t) => t.id === active.id);
    const target = over.id as TicketStatus;
    if (!ticket || ticket.status === target || !COLUMNS.includes(target)) return;
    if (!TRANSITIONS[ticket.status]?.includes(target)) {
      setError(`Transición inválida: ${ticket.status} → ${target}`);
      return;
    }
    const previous = tickets;
    setTickets((current) => current.map((t) => (t.id === ticket.id ? { ...t, status: target } : t)));
    setError('');
    try {
      const updated = await ticketService.transition(ticket.id, target, 'Usuario');
      setTickets((current) => current.map((t) => (t.id === updated.id ? updated : t)));
    } catch (e: any) {
      setTickets(previous);
      setError(e?.message ?? 'No se pudo cambiar el estado');
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div>
        <h1 style={{ margin: '0 0 4px', fontSize: 20, fontWeight: 700 }}>Kanban</h1>
        <p className="muted" style={{ margin: 0 }}>Arrastra tickets entre estados válidos.</p>
      </div>
      {error && <p className="error-text" style={{ margin: 0 }}>{error}</p>}
      {loading ? <p className="muted">Cargando…</p> : (
        <DndContext sensors={sensors} onDragEnd={onDragEnd}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(220px, 1fr))', gap: 12, overflowX: 'auto', paddingBottom: 2 }}>
            {COLUMNS.map((status) => <Column key={status} status={status} tickets={grouped[status]} />)}
          </div>
        </DndContext>
      )}
    </div>
  );
};

export default KanbanIsland;
