import React, { useEffect, useMemo, useState } from 'react';
import { DndContext, DragEndEvent, PointerSensor, useSensor, useSensors } from '@dnd-kit/core';
import { SortableContext, arrayMove, rectSortingStrategy, useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { ticketService } from '../services/ticketService';
import type { ExecutiveSummary, ReportSummary } from '../services/ticketService';
import { loadState, saveState } from '../services/storageService';
import { DEFAULT_LAYOUT, SIZE_SPAN, WIDGETS, widgetById, type WidgetSize } from '../dashboard/widgetRegistry';

const LAYOUT_KEY = 'todomark_dashboard_layout_v1';
const SIZES_KEY = 'todomark_dashboard_sizes_v1';

const CYCLE: Record<WidgetSize, WidgetSize> = { sm: 'md', md: 'lg', lg: 'sm' };

const IconDrag = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round">
    <circle cx="9" cy="6" r="1.2" fill="currentColor" stroke="none" /><circle cx="15" cy="6" r="1.2" fill="currentColor" stroke="none" />
    <circle cx="9" cy="12" r="1.2" fill="currentColor" stroke="none" /><circle cx="15" cy="12" r="1.2" fill="currentColor" stroke="none" />
    <circle cx="9" cy="18" r="1.2" fill="currentColor" stroke="none" /><circle cx="15" cy="18" r="1.2" fill="currentColor" stroke="none" />
  </svg>
);
const IconClose = () => (
  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round">
    <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
  </svg>
);

const WidgetCard: React.FC<{
  id: string;
  size: WidgetSize;
  data: { report: ReportSummary; exec: ExecutiveSummary };
  onRemove: (id: string) => void;
  onResize: (id: string) => void;
}> = ({ id, size, data, onRemove, onResize }) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const def = widgetById(id);
  if (!def) return null;

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    gridColumn: `span ${SIZE_SPAN[size]}`,
  };

  return (
    <div ref={setNodeRef} style={style} className="card" >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          <span
            {...attributes}
            {...listeners}
            style={{ cursor: 'grab', color: 'var(--ink-faint)', display: 'flex', touchAction: 'none' }}
            title="Arrastrar para reordenar"
          >
            <IconDrag />
          </span>
          <h2 style={{ fontSize: 13, fontWeight: 700, margin: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{def.title}</h2>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
          <button type="button" className="btn btn-outline" style={{ fontSize: 11, padding: '2px 8px' }} onClick={() => onResize(id)} title="Cambiar tamaño">
            {size}
          </button>
          <button
            type="button"
            aria-label="Quitar widget"
            onClick={() => onRemove(id)}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--ink-faint)', display: 'flex', padding: 4 }}
          >
            <IconClose />
          </button>
        </div>
      </div>
      {def.render(data)}
    </div>
  );
};

const DashboardBuilderPage: React.FC = () => {
  const [report, setReport] = useState<ReportSummary | null>(null);
  const [exec, setExec] = useState<ExecutiveSummary | null>(null);
  const [error, setError] = useState('');
  const [layout, setLayout] = useState<string[]>(() => loadState<string[]>(LAYOUT_KEY) ?? DEFAULT_LAYOUT);
  const [sizes, setSizes] = useState<Record<string, WidgetSize>>(() => loadState<Record<string, WidgetSize>>(SIZES_KEY) ?? {});
  const [picking, setPicking] = useState(false);
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));

  useEffect(() => {
    (async () => {
      try {
        const [r, e] = await Promise.all([ticketService.getReportSummary(), ticketService.getExecutiveSummary()]);
        setReport(r);
        setExec(e);
      } catch (err: any) {
        setError(err?.message ?? 'Error al cargar datos del dashboard');
      }
    })();
  }, []);

  useEffect(() => saveState(LAYOUT_KEY, layout), [layout]);
  useEffect(() => saveState(SIZES_KEY, sizes), [sizes]);

  const sizeOf = (id: string): WidgetSize => sizes[id] ?? widgetById(id)?.size ?? 'md';

  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return;
    setLayout((current) => {
      const oldIndex = current.indexOf(String(active.id));
      const newIndex = current.indexOf(String(over.id));
      if (oldIndex === -1 || newIndex === -1) return current;
      return arrayMove(current, oldIndex, newIndex);
    });
  };

  const removeWidget = (id: string) => setLayout((current) => current.filter((w) => w !== id));
  const addWidget = (id: string) => {
    setLayout((current) => (current.includes(id) ? current : [...current, id]));
    setPicking(false);
  };
  const cycleSize = (id: string) => setSizes((current) => ({ ...current, [id]: CYCLE[sizeOf(id)] }));
  const resetLayout = () => {
    setLayout(DEFAULT_LAYOUT);
    setSizes({});
  };

  const available = useMemo(() => WIDGETS.filter((w) => !layout.includes(w.id)), [layout]);

  if (error) return <p className="error-text">{error}</p>;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
        <div>
          <h1 style={{ margin: '0 0 4px', fontSize: 20, fontWeight: 700 }}>Mi dashboard</h1>
          <p className="muted" style={{ margin: 0 }}>
            Arrastrá para reordenar, tocá el botón de tamaño (sm/md/lg) para agrandar, quitá con la ✕. Se guarda en este navegador.
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8, position: 'relative' }}>
          <button type="button" className="btn btn-outline" onClick={resetLayout}>Restablecer</button>
          <div style={{ position: 'relative' }}>
            <button type="button" className="btn btn-primary" onClick={() => setPicking((p) => !p)} disabled={available.length === 0}>
              + Agregar widget
            </button>
            {picking && (
              <div
                className="card"
                style={{
                  position: 'absolute', right: 0, top: '110%', zIndex: 10, width: 260, maxHeight: 320, overflowY: 'auto',
                  display: 'flex', flexDirection: 'column', gap: 4, padding: 8,
                }}
              >
                {available.length === 0 && <span className="muted" style={{ padding: 8 }}>Ya agregaste todos los widgets.</span>}
                {available.map((w) => (
                  <button
                    key={w.id}
                    type="button"
                    onClick={() => addWidget(w.id)}
                    style={{
                      textAlign: 'left', background: 'none', border: 'none', cursor: 'pointer', padding: '8px 10px',
                      borderRadius: 'var(--radius-sm)', fontSize: 13,
                    }}
                    onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--surface-alt)')}
                    onMouseLeave={(e) => (e.currentTarget.style.background = 'none')}
                  >
                    {w.title}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {!report || !exec ? (
        <p className="muted">Cargando…</p>
      ) : layout.length === 0 ? (
        <p className="muted">Sin widgets. Agregá alguno con "+ Agregar widget".</p>
      ) : (
        <DndContext sensors={sensors} onDragEnd={onDragEnd}>
          <SortableContext items={layout} strategy={rectSortingStrategy}>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(12, 1fr)', gap: 12 }}>
              {layout.map((id) => (
                <WidgetCard key={id} id={id} size={sizeOf(id)} data={{ report, exec }} onRemove={removeWidget} onResize={cycleSize} />
              ))}
            </div>
          </SortableContext>
        </DndContext>
      )}
    </div>
  );
};

export default DashboardBuilderPage;
