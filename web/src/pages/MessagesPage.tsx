import React, { useEffect, useState } from 'react';
import WhatsappTargetInput from '../components/WhatsappTargetInput';
import { formatRelative, formatDateTime } from '../utils/date';

const API_URL = (window as any).__ENV?.VITE_API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8081';

type Thread = { chatId: string; chatName: string; lastText: string; lastAt: string; messageCount: number };
type Message = { id: string; chatId: string; chatName: string; direction: 'in' | 'out'; text: string; ticketId: string | null; createdAt: string };

const PAGE_SIZE = 50;

export default function MessagesPage() {
  const [threads, setThreads] = useState<Thread[]>([]);
  const [threadsHasMore, setThreadsHasMore] = useState(false);
  const [loadingMoreThreads, setLoadingMoreThreads] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [messagesHasMore, setMessagesHasMore] = useState(false);
  const [loadingMoreMessages, setLoadingMoreMessages] = useState(false);
  const [loadingThreads, setLoadingThreads] = useState(true);
  const [error, setError] = useState('');

  const [composing, setComposing] = useState(false);
  const [newTarget, setNewTarget] = useState('');
  const [text, setText] = useState('');
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState('');

  const loadThreads = async (before?: string) => {
    try {
      const url = new URL(`${API_URL}/api/messages/threads`);
      url.searchParams.set('limit', String(PAGE_SIZE));
      if (before) url.searchParams.set('before', before);
      const res = await fetch(url.toString());
      const data: Thread[] = await res.json();
      const page = Array.isArray(data) ? data : [];
      setThreads((prev) => (before ? [...prev, ...page] : page));
      setThreadsHasMore(page.length === PAGE_SIZE);
    } catch {
      setError('No se pudo cargar la bandeja');
    } finally {
      setLoadingThreads(false);
      setLoadingMoreThreads(false);
    }
  };

  const loadMoreThreads = () => {
    if (threads.length === 0) return;
    setLoadingMoreThreads(true);
    loadThreads(threads[threads.length - 1].lastAt);
  };

  useEffect(() => {
    loadThreads();
  }, []);

  const loadMessages = async (chatId: string, before?: string) => {
    if (!before) {
      setSelected(chatId);
      setComposing(false);
    }
    try {
      const url = new URL(`${API_URL}/api/messages/threads/${encodeURIComponent(chatId)}`);
      url.searchParams.set('limit', String(PAGE_SIZE));
      if (before) url.searchParams.set('before', before);
      const res = await fetch(url.toString());
      const data: Message[] = await res.json();
      const page = Array.isArray(data) ? data : [];
      setMessages((prev) => (before ? [...page, ...prev] : page));
      setMessagesHasMore(page.length === PAGE_SIZE);
    } catch {
      setError('No se pudo cargar la conversación');
    } finally {
      setLoadingMoreMessages(false);
    }
  };

  const loadMoreMessages = () => {
    if (!selected || messages.length === 0) return;
    setLoadingMoreMessages(true);
    loadMessages(selected, messages[0].createdAt);
  };

  const send = async (to: string, body: string) => {
    if (!to || !body.trim()) return;
    setSending(true);
    setSendError('');
    try {
      const res = await fetch(`${API_URL}/api/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ to, text: body }),
      });
      const data = await res.json();
      if (!res.ok) {
        setSendError(data.error ?? 'Error al enviar');
        return;
      }
      setText('');
      setNewTarget('');
      setComposing(false);
      await loadThreads();
      await loadMessages(data.chatId ?? to);
    } catch {
      setSendError('Error al contactar el API');
    } finally {
      setSending(false);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: 21, fontWeight: 700 }}>Bandeja</h1>
          <p style={{ margin: '4px 0 0', fontSize: 13, color: 'var(--ink-soft)' }}>Conversaciones de WhatsApp, ligadas o no a un ticket.</p>
        </div>
        <button type="button" className="btn btn-primary" onClick={() => { setComposing(true); setSelected(null); }}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round">
            <line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" />
          </svg>
          Nuevo mensaje
        </button>
      </div>

      {error && <p className="error-text">{error}</p>}

      <div style={{ display: 'grid', gridTemplateColumns: '280px 1fr', gap: 16, flex: 1, minHeight: 420 }}>
        <div className="table-wrap" style={{ overflow: 'auto' }}>
          {loadingThreads ? (
            <p className="muted" style={{ padding: 16 }}>Cargando…</p>
          ) : threads.length === 0 ? (
            <p className="muted" style={{ padding: 16 }}>Sin conversaciones todavía.</p>
          ) : (
            threads.map((t) => (
              <button
                key={t.chatId}
                type="button"
                onClick={() => loadMessages(t.chatId)}
                style={{
                  display: 'block', width: '100%', textAlign: 'left', padding: '12px 14px',
                  border: 'none', borderBottom: '1px solid var(--border)', cursor: 'pointer',
                  background: selected === t.chatId ? 'var(--surface-alt)' : 'transparent',
                }}
              >
                <div style={{ fontSize: 13, fontWeight: 600 }}>{t.chatName}</div>
                <div className="muted" style={{ marginTop: 2, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t.lastText}</div>
                <div className="muted" style={{ marginTop: 2, fontSize: 11 }}>{formatRelative(t.lastAt)} · {t.messageCount}</div>
              </button>
            ))
          )}
          {threadsHasMore && (
            <button type="button" className="btn btn-ghost" disabled={loadingMoreThreads} onClick={loadMoreThreads} style={{ width: '100%', justifyContent: 'center', padding: '10px 0' }}>
              {loadingMoreThreads ? 'Cargando…' : 'Cargar más'}
            </button>
          )}
        </div>

        <div className="card" style={{ display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          {composing ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12, flex: 1 }}>
              <div className="label">Destino</div>
              <WhatsappTargetInput value={newTarget} onChange={setNewTarget} />
              <div className="label">Mensaje</div>
              <textarea className="textarea" style={{ flex: 1 }} value={text} onChange={(e) => setText(e.target.value)} placeholder="Escribe el mensaje…" />
              {sendError && <p className="error-text">{sendError}</p>}
              <button type="button" className="btn btn-primary" disabled={sending || !newTarget || !text.trim()} onClick={() => send(newTarget, text)} style={{ justifyContent: 'center' }}>
                {sending ? 'Enviando…' : 'Enviar'}
              </button>
            </div>
          ) : !selected ? (
            <p className="muted" style={{ margin: 'auto' }}>Elige una conversación o escribe un mensaje nuevo.</p>
          ) : (
            <>
              <div style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column', gap: 8, paddingBottom: 12 }}>
                {messagesHasMore && (
                  <button type="button" className="btn btn-ghost" disabled={loadingMoreMessages} onClick={loadMoreMessages} style={{ alignSelf: 'center', padding: '6px 12px' }}>
                    {loadingMoreMessages ? 'Cargando…' : 'Cargar mensajes anteriores'}
                  </button>
                )}
                {messages.map((m) => (
                  <div key={m.id} style={{ alignSelf: m.direction === 'out' ? 'flex-end' : 'flex-start', maxWidth: '70%' }}>
                    <div
                      style={{
                        padding: '8px 12px', borderRadius: 'var(--radius-sm)', fontSize: 13,
                        background: m.direction === 'out' ? 'var(--primary-tint)' : 'var(--surface-alt)',
                        color: m.direction === 'out' ? 'var(--primary-dark)' : 'var(--ink)',
                      }}
                    >
                      {m.text}
                    </div>
                    <div className="muted" style={{ fontSize: 10.5, marginTop: 2, textAlign: m.direction === 'out' ? 'right' : 'left' }}>
                      {formatDateTime(m.createdAt)}
                    </div>
                  </div>
                ))}
              </div>
              <div style={{ display: 'flex', gap: 8, borderTop: '1px solid var(--border)', paddingTop: 12 }}>
                <input className="input" value={text} onChange={(e) => setText(e.target.value)} placeholder="Responder…" style={{ flex: 1 }} />
                {sendError && <p className="error-text">{sendError}</p>}
                <button type="button" className="btn btn-primary" disabled={sending || !text.trim()} onClick={() => send(selected, text)}>
                  {sending ? 'Enviando…' : 'Enviar'}
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
