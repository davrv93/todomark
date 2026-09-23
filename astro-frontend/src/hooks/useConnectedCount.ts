import { useEffect, useState } from 'react';

// Astro solo expone al cliente las variables prefijadas con PUBLIC_ (a diferencia de Vite puro,
// que expone todo lo prefijado con VITE_). Mismo patrón que services/ticketService.ts.
const API_URL =
  (typeof window !== 'undefined' && (window as any).__ENV?.PUBLIC_API_URL) ||
  import.meta.env.PUBLIC_API_URL ||
  'http://localhost:8081';

export function useConnectedCount(): number | null {
  const [count, setCount] = useState<number | null>(null);

  useEffect(() => {
    const es = new EventSource(`${API_URL}/api/presence/stream`);
    es.onmessage = (e) => {
      const n = parseInt(e.data, 10);
      if (!Number.isNaN(n)) setCount(n);
    };
    return () => es.close();
  }, []);

  return count;
}
