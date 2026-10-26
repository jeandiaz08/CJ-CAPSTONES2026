import { ModulePlaceholder } from "@/components/ModulePlaceholder";

export default function Page() {
  return (
    <ModulePlaceholder
      title="Integraciones"
      rf="RF-05, RF-08, RF-23, RF-25"
      description="Conexiones por cliente con canales (Mercado Libre, Shopify…) y couriers. Credenciales nunca visibles en listados ni registros."
      frontend="frontend-web/src/app/(app)/integraciones/page.tsx"
      backend="orders-service/internal/adapters/ (un adaptador por proveedor) y adapters/inbound/webhooks/"
      notes={["Los webhooks entran por /webhooks/{proveedor} con secreto/firma, sin JWT (DA-14).", "Mientras no haya contrato real, los adaptadores simulados se rotulan como tales."]}
    />
  );
}
