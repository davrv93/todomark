import { useState, useEffect, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Ticket, TicketStatus, Priority } from '../models/Ticket';
import { ticketService, TicketFilters } from '../services/ticketService';

export function useTicketState() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const filters: TicketFilters = {
    status: (searchParams.get('status') ?? undefined) as TicketStatus | undefined,
    priority: (searchParams.get('priority') ?? undefined) as Priority | undefined,
    query: searchParams.get('query') ?? undefined,
  };

  const refresh = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setTickets(await ticketService.getAll(filters));
    } catch (e: any) {
      setError(e?.message ?? 'Error al cargar los tickets');
    } finally {
      setLoading(false);
    }
  }, [searchParams]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const setFilters = (newFilters: Record<string, string>) => {
    const next = new URLSearchParams(searchParams);
    for (const [k, v] of Object.entries(newFilters)) {
      if (v) next.set(k, v);
      else next.delete(k);
    }
    setSearchParams(next);
  };

  return { tickets, filters, loading, error, refresh, dispatch: { setFilters, refresh } };
}
