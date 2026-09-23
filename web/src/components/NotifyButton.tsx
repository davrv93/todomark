import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { Ticket } from '../models/Ticket';
import { sendStatusChange } from '../services/notificationService';

type Props = { ticket: Ticket };

const NotifyButton: React.FC<Props> = ({ ticket }) => {
  const [state, setState] = useState<'idle' | 'sending' | 'ok' | 'error'>('idle');
  const [error, setError] = useState('');

  const handleNotify = async () => {
    setState('sending');
    setError('');
    try {
      await sendStatusChange(ticket, 'Usuario');
      setState('ok');
    } catch (err: any) {
      setState('error');
      setError(err?.response?.data?.error ?? 'Error al enviar');
    }
  };

  if (!ticket.whatsappChatId) {
    return (
      <p className="muted">
        Sin destinatario WhatsApp.{' '}
        <Link to={`/ticket/${ticket.id}/assign-phone`} style={{ color: 'var(--primary)', fontWeight: 600, textDecoration: 'none' }}>Asignar número</Link>
      </p>
    );
  }

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
      <button type="button" className="btn" onClick={handleNotify} disabled={state === 'sending'} style={{ background: 'var(--teal)', color: '#fff', minWidth: 212, justifyContent: 'center' }}>
        {state === 'sending' ? (
          <svg className="spin" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2} strokeLinecap="round">
            <circle cx="12" cy="12" r="9" opacity=".3" /><path d="M21 12a9 9 0 0 0-9-9" />
          </svg>
        ) : state === 'ok' ? (
          <svg className="draw-check" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.5} strokeLinecap="round" strokeLinejoin="round">
            <polyline points="20 6 9 17 4 12" />
          </svg>
        ) : (
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
            <line x1="22" y1="2" x2="11" y2="13" /><polygon points="22 2 15 22 11 13 2 9 22 2" />
          </svg>
        )}
        {state === 'sending' ? 'Enviando…' : state === 'ok' ? 'Enviado' : 'Notificar por WhatsApp'}
      </button>
      {state === 'error' && <span className="error-text">{error}</span>}
    </div>
  );
};

export default NotifyButton;
