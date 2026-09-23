import React, { ChangeEvent } from 'react';
import { STATUS_META } from './TicketStatusBadge';

type Props = {
  filters: Record<string, string>;
  onChange: (newFilters: Record<string, string>) => void;
};

const STATUS_OPTIONS: Array<{ value: string; label: string }> = [
  { value: '', label: 'Todos' },
  ...(Object.keys(STATUS_META) as Array<keyof typeof STATUS_META>).map((value) => ({ value, label: STATUS_META[value].label })),
];

const TicketFilters: React.FC<Props> = ({ filters, onChange }) => {
  const status = filters.status ?? '';

  const handleSelect = (e: ChangeEvent<HTMLSelectElement>) => {
    onChange({ [e.target.name]: e.target.value });
  };

  return (
    <div className="filters-bar" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
      <div className="filters-pills" style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
        {STATUS_OPTIONS.map((opt) => (
          <button
            key={opt.value || 'all'}
            type="button"
            className={`pill${status === opt.value ? ' active' : ''}`}
            onClick={() => onChange({ status: opt.value })}
          >
            {opt.label}
          </button>
        ))}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <input
          type="text"
          name="query"
          className="input search-input"
          placeholder="Buscar por título o solicitante"
          value={filters.query ?? ''}
          onChange={(e) => onChange({ query: e.target.value })}
        />
        <select name="priority" className="select" value={filters.priority ?? ''} onChange={handleSelect} style={{ width: 140 }}>
          <option value="">Toda prioridad</option>
          <option value="low">Baja</option>
          <option value="medium">Media</option>
          <option value="high">Alta</option>
          <option value="critical">Crítica</option>
        </select>
      </div>
    </div>
  );
};

export default TicketFilters;
