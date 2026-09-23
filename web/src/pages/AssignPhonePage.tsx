import React, { useState } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';

import { ticketService } from '../services/ticketService';
import { WHATSAPP_TARGET_RE } from '../utils/whatsapp';
import WhatsappTargetInput from '../components/WhatsappTargetInput';

export default function AssignPhonePage() {
  const { id } = useParams<{ id: string }>();
  const [phone, setPhone] = useState('');
  const [msg, setMsg] = useState('');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setMsg('');
    if (!WHATSAPP_TARGET_RE.test(phone)) {
      setError('Formato inválido: número (solo dígitos, código de país sin +) o JID de grupo terminado en @g.us');
      return;
    }
    setSaving(true);
    try {
      await ticketService.setRecipient(id!, phone);
      setMsg('Número asignado');
      setTimeout(() => navigate(`/ticket/${id}`), 1500);
    } catch (err: any) {
      setError(err?.message ?? 'Error al asignar');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ maxWidth: 380 }}>
      <Link to={`/ticket/${id}`} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--ink-soft)', textDecoration: 'none', marginBottom: 10 }}>
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
          <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
        </svg>
        Volver al ticket
      </Link>
      <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 16 }}>Asignar número o grupo de WhatsApp</h1>
      <p className="muted" style={{ marginTop: 0 }}>
        Con un número individual, además activas el chatbot: podrá consultar el estado de este ticket escribiendo "menu" en WhatsApp (no aplica a grupos).
      </p>
      <form onSubmit={handleSubmit} className="card form-card">
        <div className="field">
          <label htmlFor="a-phone">Número o grupo</label>
          <WhatsappTargetInput id="a-phone" value={phone} onChange={setPhone} />
        </div>
        <button type="submit" className="btn btn-primary" disabled={saving} style={{ justifyContent: 'center' }}>
          {saving ? 'Asignando…' : 'Asignar'}
        </button>
        {msg && <p className="ok-text">{msg}</p>}
        {error && <p className="error-text">{error}</p>}
      </form>
    </div>
  );
}
