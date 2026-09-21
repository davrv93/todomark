import { Ticket, TicketStatus, Priority } from '../models/Ticket';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8081';

async function http<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => undefined);
  if (!res.ok) throw new Error((data as any)?.error ?? `HTTP ${res.status}`);
  return data as T;
}

export type TicketFilters = { status?: TicketStatus; priority?: Priority; query?: string };

function qs(filters: TicketFilters): string {
  const params = new URLSearchParams();
  if (filters.status) params.set('status', filters.status);
  if (filters.priority) params.set('priority', filters.priority);
  if (filters.query) params.set('query', filters.query);
  const s = params.toString();
  return s ? `?${s}` : '';
}

export const ticketService = {
  getAll: (filters: TicketFilters = {}) => http<Ticket[]>(`/api/tickets${qs(filters)}`),
  getById: (id: string) => http<Ticket>(`/api/tickets/${id}`),
  create: (data: Pick<Ticket, 'title' | 'description' | 'priority' | 'requester'> & { assignedTo?: string | null }) =>
    http<Ticket>('/api/tickets', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<Omit<Ticket, 'id' | 'createdAt' | 'history'>>) =>
    http<Ticket>(`/api/tickets/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => http<void>(`/api/tickets/${id}`, { method: 'DELETE' }),
  transition: (id: string, newStatus: TicketStatus, user: string) =>
    http<Ticket>(`/api/tickets/${id}/transition`, { method: 'POST', body: JSON.stringify({ newStatus, user }) }),
  setRecipient: (id: string, phone: string) =>
    http<Ticket>(`/api/tickets/${id}/recipient`, { method: 'POST', body: JSON.stringify({ phone }) }),
  // Compat con llamadas existentes
  filter: (filters: TicketFilters = {}) => ticketService.getAll(filters),
};
