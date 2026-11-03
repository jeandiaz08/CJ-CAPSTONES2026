import { ModulePlaceholder } from "@/components/ModulePlaceholder";

export default function Page() {
  return <ModulePlaceholder title="Registro histórico" description="Órdenes canceladas y entregadas hace más de 365 días. Sección pendiente de implementación." frontend="src/app/(app)/ordenes/historial" backend="GET /api/orders/v1/orders?view=registro" />;
}
