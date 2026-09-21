import React, { useEffect, useState } from 'react';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8081';

export default function WhatsAppPage() {
  const [state, setState] = useState('');
  const [qr, setQr] = useState('');
  const [msg, setMsg] = useState('');
  const [loading, setLoading] = useState(false);
  const [phone, setPhone] = useState('');
  const [pairCode, setPairCode] = useState('');

  const loadStatus = async () => {
    try {
      const res = await fetch(`${API_URL}/api/whatsapp/status`);
      const data = await res.json();
      setState(data.state ?? data.error ?? 'desconocido');
    } catch {
      setState('API no disponible');
    }
  };

  useEffect(() => {
    loadStatus();
  }, []);

  const handleQR = async () => {
    setLoading(true);
    setMsg('');
    setQr('');
    try {
      const res = await fetch(`${API_URL}/api/whatsapp/qr`, { method: 'POST' });
      const data = await res.json();
      if (data.connected) {
        setState('open');
        setMsg('WhatsApp ya está conectado');
      } else if (data.qr) {
        setQr(data.qr);
        setState('connecting');
      } else {
        setMsg(data.error ?? 'No se pudo generar el QR; reintenta');
      }
    } catch {
      setMsg('Error al contactar el API');
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = async () => {
    setLoading(true);
    setMsg('');
    try {
      const res = await fetch(`${API_URL}/api/whatsapp/logout`, { method: 'POST' });
      const data = await res.json();
      if (res.ok) {
        setState(data.state ?? 'close');
        setQr('');
        setPairCode('');
        setMsg('WhatsApp desvinculado');
      } else {
        setMsg(data.error ?? 'No se pudo desvincular');
      }
    } catch {
      setMsg('Error al contactar el API');
    } finally {
      setLoading(false);
    }
  };

  const handlePair = async () => {
    setLoading(true);
    setMsg('');
    setPairCode('');
    try {
      // La sesión de emparejamiento debe estar activa: regenerar QR primero
      await fetch(`${API_URL}/api/whatsapp/qr`, { method: 'POST' });
      const res = await fetch(`${API_URL}/api/whatsapp/pair`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone }),
      });
      const data = await res.json();
      if (data.pairingCode) {
        setPairCode(data.pairingCode);
      } else {
        setMsg(data.error ?? 'No se pudo generar el código; reintenta');
      }
    } catch {
      setMsg('Error al contactar el API');
    } finally {
      setLoading(false);
    }
  };

  const stateColor = state === 'open' ? 'var(--teal-text)' : state === 'connecting' ? 'var(--amber-text)' : 'var(--ink-soft)';

  return (
    <div style={{ maxWidth: 440 }}>
      <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 4 }}>WhatsApp</h1>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginBottom: 16 }}>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--ink-soft)' }}>
          Estado de la instancia: <strong style={{ color: stateColor }}>{state || '…'}</strong>
        </p>
        <button type="button" className="btn btn-outline" onClick={handleLogout} disabled={loading || state === 'close' || state === ''}>
          {loading ? 'Desvinculando…' : 'Desvincular'}
        </button>
      </div>

      <div className="card form-card">
        <div>
          <div className="label" style={{ marginBottom: 8 }}>Código QR</div>
          <button type="button" className="btn btn-primary" onClick={handleQR} disabled={loading} style={{ justifyContent: 'center', width: '100%' }}>
            {loading ? 'Generando…' : 'Generar QR para emparejar'}
          </button>
          {qr && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, alignItems: 'center', marginTop: 12 }}>
              {qr.startsWith('data:image') ? (
                <img src={qr} alt="QR de emparejamiento" style={{ maxWidth: '100%', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border)' }} />
              ) : (
                <p style={{ wordBreak: 'break-all', fontFamily: 'monospace', fontSize: 12 }}>{qr}</p>
              )}
              <p className="muted">Abre WhatsApp &gt; Dispositivos vinculados y escanea.</p>
            </div>
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <span style={{ flex: 1, height: 1, background: 'var(--border)' }} />
          <span className="muted">o</span>
          <span style={{ flex: 1, height: 1, background: 'var(--border)' }} />
        </div>

        <div>
          <div className="label" style={{ marginBottom: 8 }}>Código de emparejamiento</div>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              className="input"
              placeholder="Tu número con país (p.ej. 34600123456)"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              style={{ flex: 1 }}
            />
            <button type="button" className="btn btn-outline-primary" onClick={handlePair} disabled={loading || !/^\d{8,15}$/.test(phone)}>
              Obtener código
            </button>
          </div>
          {pairCode && (
            <div style={{ marginTop: 12, textAlign: 'center' }}>
              <div style={{
                fontSize: 26, fontWeight: 700, fontFamily: 'monospace', letterSpacing: '0.2em',
                color: 'var(--primary-dark)', background: 'var(--primary-tint)', borderRadius: 'var(--radius-sm)', padding: '10px 0',
              }}>
                {pairCode}
              </div>
              <p className="muted" style={{ marginTop: 8 }}>En WhatsApp: Dispositivos vinculados &gt; Vincular con el número de teléfono.</p>
            </div>
          )}
        </div>

        {msg && <p className="muted">{msg}</p>}
      </div>
    </div>
  );
}
