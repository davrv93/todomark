import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { authService } from '../services/authService';

export default function LoginPage() {
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  const [error, setError] = useState('');
  const navigate = useNavigate();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const ok = authService.login(user, pass);
    if (ok) {
      navigate('/');
    } else {
      setError('Credenciales incorrectas');
    }
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', paddingTop: '8vh' }}>
      <form onSubmit={handleSubmit} className="card form-card" style={{ width: 320, position: 'relative', overflow: 'hidden' }}>
        <div style={{ position: 'absolute', inset: 0, zIndex: 0 }}>
          <span className="particle" style={{ width: 4, height: 4, top: 10, right: 30, animationDelay: '0s' }} />
          <span className="particle" style={{ width: 3, height: 3, top: 26, right: 55, animationDelay: '1.1s' }} />
          <span className="particle" style={{ width: 5, height: 5, top: 16, right: 80, animationDelay: '.5s' }} />
        </div>
        <div style={{ position: 'relative', zIndex: 1 }}>
          <div style={{ width: 32, height: 32, borderRadius: 9, background: 'var(--primary)', display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: 12 }}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round">
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </div>
          <h1 style={{ fontSize: 19, fontWeight: 700 }}>Iniciar sesión</h1>
          <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--ink-soft)' }}>Accede a TodoMark</p>
        </div>
        <div className="field" style={{ position: 'relative', zIndex: 1 }}>
          <label htmlFor="l-user">Usuario</label>
          <input id="l-user" className="input" value={user} onChange={(e) => setUser(e.target.value)} required />
        </div>
        <div className="field" style={{ position: 'relative', zIndex: 1 }}>
          <label htmlFor="l-pass">Contraseña</label>
          <input id="l-pass" className="input" type="password" value={pass} onChange={(e) => setPass(e.target.value)} required />
        </div>
        {error && <p className="error-text" style={{ position: 'relative', zIndex: 1 }}>{error}</p>}
        <button type="submit" className="btn btn-primary" style={{ position: 'relative', zIndex: 1, justifyContent: 'center' }}>Entrar</button>
      </form>
    </div>
  );
}
