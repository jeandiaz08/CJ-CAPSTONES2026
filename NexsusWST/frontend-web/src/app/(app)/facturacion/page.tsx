import { ModulePlaceholder } from "@/components/ModulePlaceholder";

export default function Page() {
  return (
    <ModulePlaceholder
      title="Facturación"
      rf="Fuera de la entrega (estructura conservada)"
      description="Cobro de servicios logísticos a las empresas cliente."
      frontend="frontend-web/src/app/(app)/facturacion/page.tsx"
      backend="backend-core/internal/modules/billing/"
      notes={["Puede consultar el historial de Órdenes (dependencia Core → Órdenes, nunca al revés)."]}
    />
  );
}
