export interface Ticket {
  id: string;
  title: string;
  description: string;
  status: TicketStatus;
  priority: Priority;
  requester: string;
  assignedTo: string | null;
  email: string | null;
  assignedTeam: string | null;
  closedAt: string | null;
  createdAt: string; // ISO string
  updatedAt: string;
  history: TicketEvent[];
  whatsappChatId: string | null;
  remoteId: string | null;
  lastSyncAt: string | null;
  remoteUpdatedAt: string | null;
}

export type TicketStatus =
  | 'open'
  | 'in_progress'
  | 'resolved'
  | 'closed'
  | 'reopened';

export type Priority = 'low' | 'medium' | 'high' | 'critical';

export interface TicketEvent {
  timestamp: string;
  user: string;
  action: string;
  from: string;
  to: string;
}
