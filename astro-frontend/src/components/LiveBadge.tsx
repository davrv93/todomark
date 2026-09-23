import React from 'react';
import { useConnectedCount } from '../hooks/useConnectedCount';

// Mismo look que web/src/components/Header.tsx (badge "N en línea"), reusado en las
// páginas públicas: ExecutiveIsland, ClientReportsIsland y el nav de landing.astro.
const LiveBadge: React.FC = () => {
  const connected = useConnectedCount();

  if (connected === null) return null;

  return (
    <span
      className="badge badge-live"
      style={{ background: 'var(--teal-tint)', color: 'var(--teal-text)' }}
      title="Clientes conectados a la plataforma ahora mismo"
    >
      <span className="badge-dot" style={{ background: 'var(--teal)' }} />
      {connected} en línea
    </span>
  );
};

export default LiveBadge;
