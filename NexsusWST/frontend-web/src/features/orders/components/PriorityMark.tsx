import { Star } from "lucide-react";

const LABEL = "Orden prioritaria";

// Estrella de prioridad. En la tabla reserva su espacio aunque la orden no sea prioritaria,
// para que los números de todas las filas queden alineados.
export function PrioritySlot({ priority }: { priority: boolean }) {
  return (
    <span className="priority-slot">
      {priority && <span className="priority-mark" role="img" aria-label={LABEL} title={LABEL}><Star size={14} fill="currentColor" aria-hidden="true" /></span>}
    </span>
  );
}

export function PriorityBadge() {
  return (
    <span className="priority-badge" role="img" aria-label={LABEL} title={LABEL}>
      <Star size={18} fill="currentColor" aria-hidden="true" />
      <span aria-hidden="true">Prioritaria</span>
    </span>
  );
}
