import React from 'react';
import { Link } from 'react-router-dom';
import { useTicketState } from '../hooks/useTicketState';
import TicketFilters from '../components/TicketFilters';
import TicketTable from '../components/TicketTable';

const TicketListPage: React.FC = () => {
  const { tickets, filters, loading, error, dispatch } = useTicketState();

  const handleFilterChange = (newFilters: Record<string, string>) => {
    dispatch.setFilters(newFilters);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
        <div>
          <h1 style={{ fontSize: 21, fontWeight: 700, letterSpacing: '-0.2px' }}>Tickets</h1>
          <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--ink-soft)' }}>
            {loading ? 'Cargando…' : `${tickets.length} ticket${tickets.length === 1 ? '' : 's'}`}
          </p>
        </div>
        <Link to="/ticket/new" className="btn btn-primary">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round">
            <line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" />
          </svg>
          Nuevo ticket
        </Link>
      </div>

      <TicketFilters filters={filters} onChange={handleFilterChange} />

      {error && <p className="error-text">{error}</p>}
      {!error && <TicketTable tickets={tickets} />}
    </div>
  );
};

export default TicketListPage;
