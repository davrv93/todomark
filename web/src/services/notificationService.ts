import axios from 'axios';
import { Ticket } from '../models/Ticket';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8081';

export async function sendWhatsApp(ticket: Ticket, message: string) {
  await axios.post(`${API_URL}/api/notify`, {
    ticketId: ticket.id,
    event: 'custom',
    chatId: ticket.whatsappChatId,
    message,
  });
}

export async function sendStatusChange(ticket: Ticket, user: string) {
  await sendWhatsApp(ticket, `Ticket ${ticket.title} - status changed by ${user}`);
}

export async function sendAssignment(ticket: Ticket, user: string) {
  await sendWhatsApp(ticket, `Ticket ${ticket.title} - assigned to ${user}`);
}
