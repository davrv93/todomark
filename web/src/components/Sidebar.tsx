import React from 'react';
import { NavLink } from 'react-router-dom';
import { authService } from '../services/authService';

const linkStyle = (isActive: boolean): React.CSSProperties => ({
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  padding: '9px 10px',
  borderRadius: 'var(--radius-sm)',
  fontSize: 13.5,
  textDecoration: 'none',
  background: isActive ? 'var(--primary-tint)' : 'transparent',
  color: isActive ? 'var(--primary-dark)' : 'var(--ink-soft)',
  fontWeight: isActive ? 600 : 500,
});

const IconList = () => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.9} strokeLinecap="round" strokeLinejoin="round">
    <line x1="8" y1="6" x2="21" y2="6" /><line x1="8" y1="12" x2="21" y2="12" /><line x1="8" y1="18" x2="21" y2="18" />
    <circle cx="3.5" cy="6" r="1.3" fill="currentColor" stroke="none" /><circle cx="3.5" cy="12" r="1.3" fill="currentColor" stroke="none" /><circle cx="3.5" cy="18" r="1.3" fill="currentColor" stroke="none" />
  </svg>
);
const IconPlus = () => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.9} strokeLinecap="round" strokeLinejoin="round">
    <line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" />
  </svg>
);
const IconChat = () => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.9} strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
  </svg>
);
const IconPhone = () => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.9} strokeLinecap="round" strokeLinejoin="round">
    <path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72c.13.96.36 1.9.7 2.81a2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45c.91.34 1.85.57 2.81.7A2 2 0 0 1 22 16.92z" />
  </svg>
);
const IconLogin = () => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.9} strokeLinecap="round" strokeLinejoin="round">
    <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" /><polyline points="10 17 15 12 10 7" /><line x1="15" y1="12" x2="3" y2="12" />
  </svg>
);
const IconLogout = () => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.9} strokeLinecap="round" strokeLinejoin="round">
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" /><polyline points="16 17 21 12 16 7" /><line x1="21" y1="12" x2="9" y2="12" />
  </svg>
);

const Sidebar: React.FC = () => (
  <nav
    className="sidebar"
    style={{
      width: 210,
      flexShrink: 0,
      boxSizing: 'border-box',
      background: 'var(--surface-alt)',
      borderRight: '1px solid var(--border)',
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'space-between',
      padding: '16px 12px',
      overflow: 'hidden',
    }}
  >
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <span className="sidebar-section-label" style={{ fontSize: 11, fontWeight: 700, letterSpacing: '.06em', color: 'var(--ink-faint)', textTransform: 'uppercase', padding: '6px 10px 8px' }}>Incidencias</span>
      <NavLink to="/" end className="sidebar-link" style={({ isActive }) => linkStyle(isActive)}>
        <IconList /><span className="sidebar-label">Tickets</span>
      </NavLink>
      <NavLink to="/ticket/new" className="sidebar-link" style={({ isActive }) => linkStyle(isActive)}>
        <IconPlus /><span className="sidebar-label">Crear ticket</span>
      </NavLink>

      <span className="sidebar-section-label" style={{ fontSize: 11, fontWeight: 700, letterSpacing: '.06em', color: 'var(--ink-faint)', textTransform: 'uppercase', padding: '16px 10px 8px' }}>Canales</span>
      <NavLink to="/whatsapp" className="sidebar-link" style={({ isActive }) => linkStyle(isActive)}>
        <IconChat /><span className="sidebar-label">WhatsApp</span>
      </NavLink>
      <NavLink to="/phone-link" className="sidebar-link" style={({ isActive }) => linkStyle(isActive)}>
        <IconPhone /><span className="sidebar-label">Vincular teléfono</span>
      </NavLink>
    </div>

    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <NavLink to="/login" className="sidebar-link" style={({ isActive }) => linkStyle(isActive)}>
        <IconLogin /><span className="sidebar-label">Sesión</span>
      </NavLink>
      <div style={{ height: 1, background: 'var(--border)', margin: '2px 4px' }} />
      <button
        type="button"
        className="sidebar-link"
        onClick={() => { authService.logout(); window.location.href = '/login'; }}
        style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '9px 10px', borderRadius: 'var(--radius-sm)', fontSize: 13.5, color: 'var(--ink-soft)', background: 'transparent', border: 'none', cursor: 'pointer', textAlign: 'left' }}
      >
        <IconLogout /><span className="sidebar-label">Cerrar sesión</span>
      </button>
    </div>
  </nav>
);

export default Sidebar;
