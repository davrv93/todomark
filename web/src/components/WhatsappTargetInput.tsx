import React, { useEffect, useState } from 'react';
import { isGroupTarget } from '../utils/whatsapp';

const API_URL = (window as any).__ENV?.VITE_API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8081';

type Group = { id: string; subject: string; size?: number };

type Props = {
  id?: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
};

const WhatsappTargetInput: React.FC<Props> = ({ id, value, onChange, placeholder }) => {
  const [mode, setMode] = useState<'individual' | 'group'>(isGroupTarget(value) ? 'group' : 'individual');
  const [groups, setGroups] = useState<Group[] | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    if (mode !== 'group' || groups !== null) return;
    fetch(`${API_URL}/api/whatsapp/groups`)
      .then((res) => {
        if (!res.ok) throw new Error();
        return res.json();
      })
      .then((data) => setGroups(Array.isArray(data) ? data : []))
      .catch(() => setError('No se pudieron cargar los grupos (¿WhatsApp está vinculado en /whatsapp?)'));
  }, [mode, groups]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <div style={{ display: 'flex', gap: 6 }}>
        <button type="button" className={`pill${mode === 'individual' ? ' active' : ''}`} onClick={() => setMode('individual')}>
          Número
        </button>
        <button type="button" className={`pill${mode === 'group' ? ' active' : ''}`} onClick={() => setMode('group')}>
          Grupo
        </button>
      </div>

      {mode === 'individual' ? (
        <input
          id={id}
          className="input"
          value={isGroupTarget(value) ? '' : value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder ?? '51987654321'}
        />
      ) : error ? (
        <p className="error-text">{error}</p>
      ) : groups === null ? (
        <p className="muted">Cargando grupos…</p>
      ) : groups.length === 0 ? (
        <p className="muted">Esta instancia de WhatsApp no está en ningún grupo.</p>
      ) : (
        <select id={id} className="select" value={isGroupTarget(value) ? value : ''} onChange={(e) => onChange(e.target.value)}>
          <option value="">Elegir grupo…</option>
          {groups.map((g) => (
            <option key={g.id} value={g.id}>
              {g.subject}
              {g.size ? ` (${g.size})` : ''}
            </option>
          ))}
        </select>
      )}
    </div>
  );
};

export default WhatsappTargetInput;
