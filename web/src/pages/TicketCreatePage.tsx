import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Building, ticketService } from '../services/ticketService';
import { PRIORITY_META } from '../components/TicketStatusBadge';
import { Priority } from '../models/Ticket';
import { WHATSAPP_TARGET_RE } from '../utils/whatsapp';
import WhatsappTargetInput from '../components/WhatsappTargetInput';

type FormState = {
  title: string;
  description: string;
  priority: Priority;
  requester: string;
  email: string;
  assignedTeam: string;
  whatsappPhone: string;
  category: string;
  buildingId: string;
  unitId: string;
};

const PRIORITIES = Object.keys(PRIORITY_META) as Priority[];
// Taxonomía simple para U4/O2 (reportes por categoría) — sin catálogo externo, se puede ampliar
// libremente desde acá si hace falta otra categoría.
const CATEGORIES = ['plomeria', 'electricidad', 'mantenimiento', 'limpieza', 'seguridad', 'administrativo', 'otro'];
const CATEGORY_LABELS: Record<string, string> = {
  plomeria: 'Plomería',
  electricidad: 'Electricidad',
  mantenimiento: 'Mantenimiento',
  limpieza: 'Limpieza',
  seguridad: 'Seguridad',
  administrativo: 'Administrativo',
  otro: 'Otro',
};

const TicketCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const [form, setForm] = useState<FormState>({
    title: '',
    description: '',
    priority: 'low',
    requester: '',
    email: '',
    assignedTeam: '',
    whatsappPhone: '',
    category: '',
    buildingId: '',
    unitId: '',
  });
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [buildings, setBuildings] = useState<Building[]>([]);

  useEffect(() => {
    ticketService.getBuildings().then(setBuildings).catch(() => {});
  }, []);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: value }));
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
      const created = await ticketService.create({
        title: form.title,
        description: form.description,
        priority: form.priority,
        requester: form.requester,
        assignedTo: null,
        email: form.email || null,
        assignedTeam: form.assignedTeam || null,
        category: form.category || null,
        buildingId: form.buildingId || null,
        unitId: form.unitId || null,
      });
      if (form.whatsappPhone) {
        await ticketService.setRecipient(created.id, form.whatsappPhone);
      }
      navigate('/');
    } catch (err: any) {
      setError(err?.message ?? 'Error al crear el ticket');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div>
        <Link to="/" style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--ink-soft)', textDecoration: 'none' }}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
          Volver a tickets
        </Link>
        <h1 style={{ margin: '10px 0 4px', fontSize: 20, fontWeight: 700 }}>Crear ticket</h1>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--ink-soft)' }}>Registra una nueva incidencia y asígnala a un agente.</p>
      </div>

      <form onSubmit={handleSubmit} className="card form-card">
        <div className="field">
          <label htmlFor="c-title">Título</label>
          <input id="c-title" className="input" name="title" value={form.title} onChange={handleChange} required placeholder="Ej. No llegan notificaciones de WhatsApp" />
        </div>
        <div className="field">
          <label htmlFor="c-desc">Descripción</label>
          <textarea id="c-desc" className="textarea" name="description" rows={4} value={form.description} onChange={handleChange} required placeholder="Describe el problema con el mayor detalle posible..." />
        </div>
        <div className="field">
          <label>Prioridad</label>
          <div style={{ display: 'flex', gap: 6 }}>
            {PRIORITIES.map((p) => (
              <button
                key={p}
                type="button"
                className={`seg-btn${form.priority === p ? ' active' : ''}`}
                onClick={() => setForm((prev) => ({ ...prev, priority: p }))}
              >
                {PRIORITY_META[p].label}
              </button>
            ))}
          </div>
        </div>
        <div className="two-col-grid">
          <div className="field">
            <label htmlFor="c-req">Solicitante</label>
            <input id="c-req" className="input" name="requester" value={form.requester} onChange={handleChange} required />
          </div>
          <div className="field">
            <label htmlFor="c-email">Email</label>
            <input id="c-email" className="input" name="email" type="email" value={form.email} onChange={handleChange} placeholder="solicitante@empresa.com" />
          </div>
        </div>
        <div className="field">
          <label htmlFor="c-category">Categoría (opcional)</label>
          <select
            id="c-category"
            className="input"
            name="category"
            value={form.category}
            onChange={(e) => setForm((prev) => ({ ...prev, category: e.target.value }))}
          >
            <option value="">Sin categoría</option>
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>{CATEGORY_LABELS[c]}</option>
            ))}
          </select>
        </div>
        <div className="two-col-grid">
          <div className="field">
            <label htmlFor="c-building">Edificio (demo, opcional)</label>
            <select
              id="c-building"
              className="input"
              name="buildingId"
              value={form.buildingId}
              onChange={(e) => setForm((prev) => ({ ...prev, buildingId: e.target.value }))}
            >
              <option value="">Sin edificio</option>
              {buildings.map((b) => (
                <option key={b.id} value={b.id}>{b.name}</option>
              ))}
            </select>
          </div>
          <div className="field">
            <label htmlFor="c-unit">Unidad (opcional)</label>
            <input id="c-unit" className="input" name="unitId" value={form.unitId} onChange={handleChange} placeholder="Ej. 3B" />
          </div>
        </div>
        <div className="field">
          <label htmlFor="c-team">Equipo asignado</label>
          <input id="c-team" className="input" name="assignedTeam" value={form.assignedTeam} onChange={handleChange} placeholder="Mesa de ayuda, Soporte N2..." />
        </div>
        <div className="field">
          <label htmlFor="c-phone">WhatsApp del solicitante (opcional)</label>
          <WhatsappTargetInput
            id="c-phone"
            value={form.whatsappPhone}
            onChange={(v) => setForm((prev) => ({ ...prev, whatsappPhone: v }))}
          />
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
          <Link to="/" className="btn btn-outline">Cancelar</Link>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round">
              <polyline points="20 6 9 17 4 12" />
            </svg>
            {saving ? 'Creando…' : 'Crear ticket'}
          </button>
        </div>
        {error && <p className="error-text">{error}</p>}
      </form>
    </div>
  );
};

export default TicketCreatePage;
