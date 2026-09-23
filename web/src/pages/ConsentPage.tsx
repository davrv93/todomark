import React, { useEffect, useState } from 'react';

const API_URL = (window as any).__ENV?.VITE_API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8081';

export default function ConsentPage() {
  const [error, setError] = useState('');

  useEffect(() => {
    const challenge = new URLSearchParams(window.location.search).get('consent_challenge');
    if (!challenge) {
      setError('Falta consent_challenge en la URL');
      return;
    }
    (async () => {
      try {
        const res = await fetch(`${API_URL}/api/hydra/consent`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ challenge }),
        });
        const data = await res.json();
        if (!res.ok) {
          setError(data.error ?? 'Error al autorizar');
          return;
        }
        window.location.href = data.redirect_to;
      } catch {
        setError('Error al contactar el API');
      }
    })();
  }, []);

  return (
    <div style={{ display: 'flex', justifyContent: 'center', paddingTop: '8vh' }}>
      <div className="card" style={{ width: 320, textAlign: 'center' }}>
        {error ? <p className="error-text">{error}</p> : <p className="muted">Autorizando…</p>}
      </div>
    </div>
  );
}
