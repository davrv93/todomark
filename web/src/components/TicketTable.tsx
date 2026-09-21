import React from 'react';
import { Ticket } from '../models/Ticket';
import TicketRow from './TicketRow';

type Props = {
  tickets: Ticket[];
};

const TicketTable: React.FC<Props> = ({ tickets }) => {
  if (tickets.length === 0) {
    return (
      <div className="table-wrap" style={{ padding: '32px 16px', textAlign: 'center' }}>
        <span className="muted">No hay tickets que coincidan con los filtros.</span>
      </div>
    );
  }

  return (
    <div className="table-wrap">
      <table className="ticket-table">
        <thead>
          <tr>
            <th>Ticket</th>
            <th>Estado</th>
            <th>Prioridad</th>
            <th>Solicitante</th>
            <th>Asignado</th>
            <th>Actualizado</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {tickets.map((t) => (
            <TicketRow key={t.id} ticket={t} />
          ))}
        </tbody>
      </table>
    </div>
  );
};

export default TicketTable;
