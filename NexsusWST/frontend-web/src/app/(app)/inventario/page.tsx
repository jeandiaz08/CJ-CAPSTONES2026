import { ModulePlaceholder } from "@/components/ModulePlaceholder";

export default function Page() {
  return (
    <ModulePlaceholder
      title="Inventario"
      rf="Fuera de la entrega (estructura conservada)"
      description="Consulta de stock por SKU: resumen sincronizado desde Invas y stock comprometido por Nexus (solo lectura)."
      frontend="frontend-web/src/app/(app)/inventario/page.tsx"
      backend="backend-core/internal/modules/inventory/"
      notes={["Invas es la fuente de verdad del stock; Nexus no modifica ni replica lotes.", "El comprometido lo escribe solo Órdenes (tabla stock_committed); el Core lo lee."]}
    />
  );
}
