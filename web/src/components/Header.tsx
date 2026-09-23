import React from 'react';
import { useConnectedCount } from '../hooks/useConnectedCount';
import { useInView } from '../hooks/useInView';

const Header: React.FC = () => {
  const connected = useConnectedCount();
  const { ref: particleRef, inView } = useInView<HTMLDivElement>();

  return (
  <header
    style={{
      height: 60,
      flexShrink: 0,
      boxSizing: 'border-box',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '0 24px',
      background: 'var(--surface)',
      borderBottom: '1px solid var(--border)',
      position: 'relative',
      overflow: 'hidden',
    }}
  >
    <div ref={particleRef} className="particle-field" data-paused={!inView} style={{ position: 'absolute', inset: 0, zIndex: 0 }}>
      <span className="particle" style={{ width: 4, height: 4, top: 14, left: '42%', animationDelay: '0s' }} />
      <span className="particle" style={{ width: 3, height: 3, top: 36, left: '46%', animationDelay: '1.2s' }} />
      <span className="particle particle-extra" style={{ width: 5, height: 5, top: 20, left: '50%', animationDelay: '.6s' }} />
      <span className="particle particle-extra" style={{ width: 3, height: 3, top: 40, left: '54%', animationDelay: '2s' }} />
    </div>

    <div style={{ display: 'flex', alignItems: 'center', gap: 12, position: 'relative', zIndex: 1 }}>
      <div className="brand-glow" />
      <div style={{ width: 30, height: 30, borderRadius: 8, background: 'var(--primary)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round">
          <polyline points="20 6 9 17 4 12" />
        </svg>
      </div>
      <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 17, letterSpacing: '-0.2px' }}>TodoMark</span>
      <span style={{ width: 1, height: 16, background: 'var(--border)' }} />
      <span className="header-subtitle" style={{ fontSize: 12.5, color: 'var(--ink-soft)' }}>Gestión de incidencias</span>
    </div>

    <div style={{ display: 'flex', alignItems: 'center', gap: 6, position: 'relative', zIndex: 1 }}>
      {connected !== null && (
        <span
          className="badge badge-live"
          style={{ background: 'var(--teal-tint)', color: 'var(--teal-text)', marginRight: 4 }}
          title="Clientes conectados a la plataforma ahora mismo"
        >
          <span className="badge-dot" style={{ background: 'var(--teal)' }} />
          {connected} en línea
        </span>
      )}
      <button type="button" aria-label="Buscar" className="icon-btn" style={{ width: 32, height: 32, borderRadius: 'var(--radius-sm)', border: '1px solid var(--border)', background: 'var(--surface)', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', color: 'var(--ink-soft)' }}>
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
          <circle cx="11" cy="11" r="7" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
      </button>
      <button type="button" aria-label="Notificaciones" className="icon-btn" style={{ width: 32, height: 32, borderRadius: 'var(--radius-sm)', border: '1px solid var(--border)', background: 'var(--surface)', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', color: 'var(--ink-soft)', position: 'relative' }}>
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
          <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        <span style={{ position: 'absolute', top: 5, right: 6, width: 6, height: 6, borderRadius: '50%', background: 'var(--red)', border: '1.5px solid var(--surface)' }} />
      </button>
      <span style={{ width: 1, height: 20, background: 'var(--border)', margin: '0 4px' }} />
      <div className="avatar" style={{ width: 26, height: 26 }}>SA</div>
    </div>
  </header>
  );
};

export default Header;
