import { Ticket, TicketStatus, Priority } from '../models/Ticket';

export const API_URL = (window as any).__ENV?.VITE_API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8081';

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
export type WeeklyBucket = { weekStart: string; created: number; closed: number };
export type Building = { id: string; name: string; tier: string };
export type Attachment = { id: string; ticketId: string; kind: 'photo' | 'audio'; mimeType: string; fileName: string; transcript: string | null; source: string; createdAt: string };
export type ReportChatTable = { columns: string[]; rows: string[][] };
export type ReportChatChart = { type: 'bar' | 'line' | 'doughnut'; labels: string[]; datasets: { label: string; data: number[] }[] };
export type ReportChatReply = { reply: string; table: ReportChatTable | null; chart: ReportChatChart | null };
export type BuildingStat = { buildingId: string; name: string; tier: string; count: number; avgResolutionHours: number | null };
export type CategoryByBuilding = { buildingId: string; buildingName: string; category: string; count: number };
export type ReportSummary = {
  byStatus: Record<string, number>;
  byPriority: Record<string, number>;
  byChannel: Record<string, number>;
  byAgent: Record<string, number>;
  byCategory: Record<string, number>;
  dailyTickets: Record<string, number>;
  averageResolutionHours: number | null;
  agingHoursByStatus: Record<string, number>;
  unassignedCount: number;
  unassignedOldestHours: number | null;
  reopensToday: number;
  slaAtRiskCount: number;
  slaOverdueCount: number;
  frtHours: number | null;
  weeklyBacklog: WeeklyBucket[];
  byBuilding: BuildingStat[];
  categoryByBuilding: CategoryByBuilding[];
};
export type ExecutiveSummary = {
  openNow: number;
  netChange7d: number;
  mttrHours: number | null;
  reopenRatePct: number | null;
  csatAvg: number | null;
  slaCompliancePct: number | null;
  weeklyTrend: WeeklyBucket[];
  costEstimateDemo: number | null;
  landingVisits: number;
  leadsTotal: number;
};

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
  getByEmail: (email: string) => http<Ticket[]>(`/api/tickets/email/${encodeURIComponent(email)}`),
  getByTeam: (team: string) => http<Ticket[]>(`/api/tickets/team/${encodeURIComponent(team)}`),
  getReportSummary: () => http<ReportSummary>('/api/reports/summary'),
  getExecutiveSummary: () => http<ExecutiveSummary>('/api/reports/executive'),
  create: (data: Pick<Ticket, 'title' | 'description' | 'priority' | 'requester'> & { assignedTo?: string | null; email?: string | null; assignedTeam?: string | null; category?: string | null; buildingId?: string | null; unitId?: string | null }) =>
    http<Ticket>('/api/tickets', { method: 'POST', body: JSON.stringify(data) }),
  getBuildings: () => http<Building[]>('/api/buildings'),
  getAttachments: (ticketId: string) => http<Attachment[]>(`/api/attachments?ticketId=${encodeURIComponent(ticketId)}`),
  reportChat: (message: string, provider: 'gemini' | 'deepseek') =>
    http<ReportChatReply>('/api/report-chat', { method: 'POST', body: JSON.stringify({ message, provider }) }),
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
