import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { sendWhatsApp } from '../services/notificationService';
import { ticketService } from '../services/ticketService';
import { WHATSAPP_TARGET_RE } from '../utils/whatsapp';

export default function PhoneLinkPage() {
  const navigate = useNavigate();
  const [ticketId, setTicketId] = useState('');
  const [phone, setPhone] = useState('');
  const [message, setMessage] = useState('');
  const [sending, setSending] = useState(false);
  const [status, setStatus] = useState<{ type: 'ok' | 'error'; text: string } | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setStatus(null);
    if (!WHATSAPP_TARGET_RE.test(phone)) {
      setStatus({ type: 'error', text: 'Formato inválido: número (solo dígitos, código de país sin +) o JID de grupo terminado en @g.us' });
      return;
    }
    setSending(true);
    try {
      const ticket = await ticketService.getById(ticketId);
      if (!ticket) {
        setStatus({ type: 'error', text: 'Ticket no encontrado' });
        return;
      }
      await sendWhatsApp({ ...ticket, whatsappChatId: phone }, message || `Ticket ${ticket.title}`);
      setStatus({ type: 'ok', text: 'Mensaje enviado' });
      setTimeout(() => navigate(`/ticket/${ticketId}`), 1500);
    } catch (err: any) {
      setStatus({ type: 'error', text: err?.response?.data?.error ?? err?.message ?? 'Error al enviar' });
    } finally {
      setSending(false);
    }
  };

  return (
    <div style={{ maxWidth: 420 }}>
      <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 4 }}>Vincular número de WhatsApp</h1>
      <p style={{ margin: '0 0 16px', fontSize: 13, color: 'var(--ink-soft)' }}>Envía un mensaje directo asociando un número a un ticket.</p>
      <form onSubmit={handleSubmit} className="card form-card">
        <div className="field">
          <label htmlFor="p-id">ID de ticket</label>
          <input id="p-id" className="input" value={ticketId} onChange={(e) => setTicketId(e.target.value)} required />
        </div>
        <div className="field">
          <label htmlFor="p-phone">Número o grupo de WhatsApp</label>
          <input id="p-phone" className="input" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="51987654321 o ID de grupo (…@g.us)" required />
        </div>
        <div className="field">
          <label htmlFor="p-msg">Mensaje</label>
          <textarea id="p-msg" className="textarea" value={message} placeholder="Opcional: mensaje personalizado" onChange={(e) => setMessage(e.target.value)} />
        </div>
        <button type="submit" className="btn btn-primary" disabled={sending} style={{ justifyContent: 'center' }}>
          {sending ? 'Enviando…' : 'Enviar'}
        </button>
        {status && <p className={status.type === 'ok' ? 'ok-text' : 'error-text'}>{status.text}</p>}
      </form>
    </div>
  );
}
