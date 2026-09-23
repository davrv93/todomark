import React, { useRef, useState } from 'react';

type Props = { onClose: () => void; children: React.ReactNode };

const CLOSE_THRESHOLD = 90;

/**
 * En desktop es un passthrough (sin estilos propios fuera de la media query mobile).
 * En mobile (<=860px, ver theme.css) se vuelve un sheet full-screen que se puede
 * cerrar arrastrando el handle hacia abajo.
 */
const MobileSheet: React.FC<Props> = ({ onClose, children }) => {
  const [dragY, setDragY] = useState(0);
  const dragging = useRef(false);
  const startY = useRef(0);

  const onPointerDown = (e: React.PointerEvent) => {
    dragging.current = true;
    startY.current = e.clientY;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  };

  const onPointerMove = (e: React.PointerEvent) => {
    if (!dragging.current) return;
    setDragY(Math.max(0, e.clientY - startY.current));
  };

  const endDrag = () => {
    if (!dragging.current) return;
    dragging.current = false;
    if (dragY > CLOSE_THRESHOLD) {
      onClose();
    } else {
      setDragY(0);
    }
  };

  return (
    <div
      className="mobile-sheet"
      style={dragY ? { transform: `translateY(${dragY}px)`, transition: 'none' } : undefined}
    >
      <div
        className="mobile-sheet-handle"
        role="button"
        tabIndex={0}
        aria-label="Cerrar"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onClose();
          }
        }}
      />
      <div className="mobile-sheet-inner">{children}</div>
    </div>
  );
};

export default MobileSheet;
