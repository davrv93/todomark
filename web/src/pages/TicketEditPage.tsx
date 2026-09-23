import React, { useState, useEffect } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { ticketService } from '../services/ticketService';
import { PRIORITY_META } from '../components/TicketStatusBadge';
import { Priority } from '../models/Ticket';
import { WHATSAPP_TARGET_RE } from '../utils/whatsapp';
import WhatsappTargetInput from '../components/WhatsappTargetInput';

type FormState = {
  title: string;
  description: string;
  priority: Priority;
  requester: string;
  assignedTo: string | null;
  email: string;
  assignedTeam: string;
  whatsappPhone: string;
};

const PRIORITIES = Object.keys(PRIORITY_META) as Priority[];

const TicketEditPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [form, setForm] = useState<FormState | null>(null);
  const [initialPhone, setInitialPhone] = useState('');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const t = await ticketService.getById(id ?? '');
        setForm({
          title: t.title,
          description: t.description,
          priority: t.priority,
          requester: t.requester,
          assignedTo: t.assignedTo,
          email: t.email ?? '',
          assignedTeam: t.assignedTeam ?? '',
          whatsappPhone: t.whatsappChatId ?? '',
        });
        setInitialPhone(t.whatsappChatId ?? '');
      } catch (e: any) {
        setError(e?.message ?? 'Error al cargar el ticket');
      }
    })();
  }, [id]);

  if (error) return <p className="error-text">{error}</p>;
  if (!form) return <p className="muted">Cargando…</p>;

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setForm((prev) => (prev ? { ...prev, [name]: value } : prev));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    if (form.whatsappPhone && !WHATSAPP_TARGET_RE.test(form.whatsappPhone)) {
      setError('Destino inválido: número (solo dígitos, código de país sin +) o JID de grupo terminado en @g.us');
      return;
    }
    setSaving(true);
    try {
      await ticketService.update(id ?? '', {
        title: form.title,
        description: form.description,
        priority: form.priority,
        requester: form.requester,
        assignedTo: form.assignedTo || null,
        email: form.email || null,
        assignedTeam: form.assignedTeam || null,
      });
      if (form.whatsappPhone && form.whatsappPhone !== initialPhone) {
        await ticketService.setRecipient(id ?? '', form.whatsappPhone);
      }
      navigate(`/ticket/${id}`);
    } catch (err: any) {
      setError(err?.message ?? 'Error al guardar');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div>
        <Link to={`/ticket/${id}`} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--ink-soft)', textDecoration: 'none' }}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
          Volver al ticket
        </Link>
        <h1 style={{ margin: '10px 0 4px', fontSize: 20, fontWeight: 700 }}>Editar ticket</h1>
      </div>

      <form onSubmit={handleSubmit} className="card form-card">
        <div className="field">
          <label htmlFor="e-title">Título</label>
          <input id="e-title" className="input" name="title" value={form.title} onChange={handleChange} required />
        </div>
        <div className="field">
          <label htmlFor="e-desc">Descripción</label>
          <textarea id="e-desc" className="textarea" name="description" rows={4} value={form.description} onChange={handleChange} required />
        </div>
        <div className="field">
          <label>Prioridad</label>
          <div style={{ display: 'flex', gap: 6 }}>
            {PRIORITIES.map((p) => (
              <button
                key={p}
                type="button"
                className={`seg-btn${form.priority === p ? ' active' : ''}`}
                onClick={() => setForm((prev) => (prev ? { ...prev, priority: p } : prev))}
              >
                {PRIORITY_META[p].label}
              </button>
            ))}
          </div>
        </div>
        <div className="two-col-grid">
          <div className="field">
            <label htmlFor="e-req">Solicitante</label>
            <input id="e-req" className="input" name="requester" value={form.requester} onChange={handleChange} required />
          </div>
          <div className="field">
            <label htmlFor="e-email">Email</label>
            <input id="e-email" className="input" name="email" type="email" value={form.email} onChange={handleChange} placeholder="solicitante@empresa.com" />
          </div>
        </div>
        <div className="two-col-grid">
          <div className="field">
            <label htmlFor="e-assign">Asignado a</label>
            <input id="e-assign" className="input" name="assignedTo" value={form.assignedTo ?? ''} onChange={handleChange} placeholder="Sin asignar" />
          </div>
          <div className="field">
            <label htmlFor="e-team">Equipo asignado</label>
            <input id="e-team" className="input" name="assignedTeam" value={form.assignedTeam} onChange={handleChange} placeholder="Mesa de ayuda, Soporte N2..." />
          </div>
        </div>
        <div className="field">
          <label htmlFor="e-phone">WhatsApp del solicitante</label>
          <WhatsappTargetInput
            id="e-phone"
            value={form.whatsappPhone}
            onChange={(v) => setForm((prev) => (prev ? { ...prev, whatsappPhone: v } : prev))}
          />
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
          <Link to={`/ticket/${id}`} className="btn btn-outline">Cancelar</Link>
          <button type="submit" className="btn btn-primary" disabled={saving}>{saving ? 'Guardando…' : 'Guardar'}</button>
        </div>
        {error && <p className="error-text">{error}</p>}
      </form>
    </div>
  );
};

export default TicketEditPage;
