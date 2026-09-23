import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from 'react-oidc-context';
import { authService } from '../services/authService';
import { isHydraConfigured } from '../components/HydraOAuth';
import { useInView } from '../hooks/useInView';

const API_URL = (window as any).__ENV?.VITE_API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8081';

export default function LoginPage() {
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [forceManual, setForceManual] = useState(false);
  const navigate = useNavigate();
  const auth = useAuth();
  const { ref: particleRef, inView } = useInView<HTMLDivElement>();

  const loginChallenge = new URLSearchParams(window.location.search).get('login_challenge');

  useEffect(() => {
    if (isHydraConfigured && !loginChallenge && auth?.isAuthenticated) {
      navigate('/', { replace: true });
    }
  }, [auth?.isAuthenticated, navigate, loginChallenge]);

  const handleSso = async () => {
    setError('');
    try {
      await auth?.signinRedirect();
    } catch (err: any) {
      setError(err?.message ?? 'No se pudo iniciar el login con Hydra');
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (loginChallenge) {
      setSubmitting(true);
      try {
        const res = await fetch(`${API_URL}/api/hydra/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ challenge: loginChallenge, username: user, password: pass }),
        });
        const data = await res.json();
        if (!res.ok) {
          setError(data.error ?? 'Credenciales incorrectas');
          return;
        }
        window.location.href = data.redirect_to;
      } catch {
        setError('Error al contactar el API');
      } finally {
        setSubmitting(false);
      }
      return;
    }

    const ok = authService.login(user, pass);
    if (ok) {
      navigate('/');
    } else {
      setError('Credenciales incorrectas');
    }
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', paddingTop: '8vh' }}>
      <form onSubmit={handleSubmit} className="card form-card login-card-enter" style={{ width: 320, position: 'relative', overflow: 'hidden' }}>
        <div ref={particleRef} className="particle-field" data-paused={!inView} style={{ position: 'absolute', inset: 0, zIndex: 0 }}>
          <span className="particle" style={{ width: 4, height: 4, top: 10, right: 30, animationDelay: '0s' }} />
          <span className="particle" style={{ width: 3, height: 3, top: 26, right: 55, animationDelay: '1.1s' }} />
          <span className="particle particle-extra" style={{ width: 5, height: 5, top: 16, right: 80, animationDelay: '.5s' }} />
        </div>
        <div className="field-enter" style={{ position: 'relative', zIndex: 1, animationDelay: '40ms' }}>
          <div className="brand-glow" style={{ left: -20, top: 20 }} />
          <div style={{ width: 32, height: 32, borderRadius: 9, background: 'var(--primary)', display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: 12, position: 'relative' }}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round">
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </div>
          <h1 style={{ fontSize: 19, fontWeight: 700 }}>Iniciar sesión</h1>
          <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--ink-soft)' }}>Accede a TodoMark</p>
        </div>
        {isHydraConfigured && !loginChallenge && !forceManual ? (
          <>
            <button
              type="button"
              className="btn btn-primary"
              style={{ position: 'relative', zIndex: 1, justifyContent: 'center' }}
              onClick={handleSso}
            >
              Entrar con SSO
            </button>
            {error && <p className="error-text" style={{ position: 'relative', zIndex: 1 }}>{error}</p>}
            <button
              type="button"
              onClick={() => setForceManual(true)}
              className="muted field-enter"
              style={{ position: 'relative', zIndex: 1, marginTop: 10, background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline', font: 'inherit' }}
            >
              O entrar con usuario y contraseña
            </button>
          </>
        ) : (
          <>
            <div className="field field-enter" style={{ position: 'relative', zIndex: 1, animationDelay: '90ms' }}>
              <label htmlFor="l-user">Usuario</label>
              <input id="l-user" className="input" value={user} onChange={(e) => setUser(e.target.value)} required />
            </div>
            <div className="field field-enter" style={{ position: 'relative', zIndex: 1, animationDelay: '140ms' }}>
              <label htmlFor="l-pass">Contraseña</label>
              <input id="l-pass" className="input" type="password" value={pass} onChange={(e) => setPass(e.target.value)} required />
            </div>
            {error && <p className="error-text" style={{ position: 'relative', zIndex: 1 }}>{error}</p>}
            <button type="submit" className="btn btn-primary field-enter" disabled={submitting} style={{ position: 'relative', zIndex: 1, justifyContent: 'center', animationDelay: '190ms' }}>
              {submitting ? 'Verificando…' : 'Entrar'}
            </button>
            <p className="muted field-enter" style={{ position: 'relative', zIndex: 1, marginTop: 10, lineHeight: 1.5, animationDelay: '220ms' }}>
              Demo: admin/user · agente/agente123 · gerente/gerente123 · viewer/viewer123
            </p>
          </>
        )}
      </form>
    </div>
  );
}
