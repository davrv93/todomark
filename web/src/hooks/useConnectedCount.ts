import { useEffect, useState } from 'react';

const API_URL = (window as any).__ENV?.VITE_API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8081';

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
