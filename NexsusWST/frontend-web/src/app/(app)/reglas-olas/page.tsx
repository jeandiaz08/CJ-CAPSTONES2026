import { ModulePlaceholder } from "@/components/ModulePlaceholder";

export default function Page() {
  return (
    <ModulePlaceholder
      title="Reglas de asignación a olas"
      rf="RF-17"
      description="Reglas con condiciones opcionales (courier, empresa, canal, tipo de despacho, bodega), prioridad y estado activa/pausada. Gana la primera coincidencia; sin regla, la orden queda Validada sin ola."
      frontend="frontend-web/src/app/(app)/reglas-olas/page.tsx"
      backend="orders-service/internal/ (nueva tabla wave_rules + caso de uso en application/)"
      notes={["Patrón Estrategia (PA-03): una política por tipo de condición, sin duplicar código.", "Los cortes horarios los gestiona Operaciones; Nexus no los automatiza (DA-10)."]}
    />
  );
}
