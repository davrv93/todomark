import React, { useEffect, useState } from 'react';

const API_URL = (window as any).__ENV?.VITE_API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8081';

type ProviderKeyCardProps = {
  id: string;
  title: string;
  description: React.ReactNode;
  endpoint: string;
  placeholder: string;
};

const ProviderKeyCard: React.FC<ProviderKeyCardProps> = ({ id, title, description, endpoint, placeholder }) => {
  const [configured, setConfigured] = useState<boolean | null>(null);
  const [apiKey, setApiKey] = useState('');
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [error, setError] = useState('');

  const loadStatus = async () => {
    try {
      const res = await fetch(`${API_URL}${endpoint}`);
      const data = await res.json();
      setConfigured(Boolean(data.configured));
    } catch {
      setError('No se pudo consultar el estado');
    }
  };

  useEffect(() => {
    loadStatus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setMsg('');
    setSaving(true);
    try {
      const res = await fetch(`${API_URL}${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ apiKey }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? 'Error al guardar');
        return;
      }
      setConfigured(true);
      setApiKey('');
      setMsg('Guardado, ya está en uso.');
    } catch {
      setError('Error al contactar el API');
    } finally {
      setSaving(false);
    }
  };

  const handleRemove = async () => {
    setError('');
    setMsg('');
    setSaving(true);
    try {
      const res = await fetch(`${API_URL}${endpoint}`, { method: 'DELETE' });
      if (!res.ok && res.status !== 204) {
        setError('Error al quitar');
        return;
      }
      setConfigured(false);
      setMsg('Key eliminada.');
    } catch {
      setError('Error al contactar el API');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="card form-card">
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{ fontSize: 14, fontWeight: 700 }}>{title}</div>
        {configured !== null && (
          <span className="badge" style={configured ? { background: 'var(--teal-tint)', color: 'var(--teal-text)' } : { background: 'var(--surface-alt)', color: 'var(--ink-soft)' }}>
            {configured ? 'Configurado' : 'No configurado'}
          </span>
        )}
      </div>
      <p className="muted" style={{ marginTop: -8 }}>{description}</p>

      <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div className="field">
          <label htmlFor={id}>API key</label>
          <input
            id={id}
            className="input"
            type="password"
            autoComplete="off"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={placeholder}
            required
          />
        </div>
        {error && <p className="error-text">{error}</p>}
        {msg && <p className="ok-text">{msg}</p>}
        <div style={{ display: 'flex', gap: 8 }}>
          <button type="submit" className="btn btn-primary" disabled={saving || !apiKey.trim()}>
            {saving ? 'Guardando…' : 'Guardar'}
          </button>
          {configured && (
            <button type="button" className="btn btn-outline" disabled={saving} onClick={handleRemove}>
              Quitar
            </button>
          )}
        </div>
      </form>
    </div>
  );
};

export default function SettingsPage() {
  return (
    <div style={{ maxWidth: 480, display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div>
        <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 4 }}>Configuración</h1>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--ink-soft)' }}>Integraciones del chatbot de la landing (<code>/landing</code>), guardadas desde acá — no hace falta tocar variables de entorno.</p>
      </div>

      <ProviderKeyCard
        id="s-gemini-key"
        title="Gemini Flash Lite (motor principal)"
        description="Motor principal del chatbot: clasifica cada mensaje (demo/soporte/queja/otro) y responde usando contexto recuperado (RAG) de la base de conocimiento. Sin key, el chatbot cae a DeepSeek si está configurado, o al mensaje de contacto."
        endpoint="/api/settings/gemini"
        placeholder="AIza..."
      />

      <ProviderKeyCard
        id="s-deepseek-key"
        title="DeepSeek (respaldo)"
        description="Motor de respaldo: se usa solo si Gemini no está configurado. Sin ninguna de las dos, el chatbot responde con un mensaje de contacto en vez de IA real."
        endpoint="/api/settings/deepseek"
        placeholder="sk-..."
      />
    </div>
  );
}
