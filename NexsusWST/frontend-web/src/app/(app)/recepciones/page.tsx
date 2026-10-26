import { ModulePlaceholder } from "@/components/ModulePlaceholder";

export default function Page() {
  return (
    <ModulePlaceholder
      title="Recepciones"
      rf="Fuera de la entrega (estructura conservada)"
      description="Ingreso de mercadería a bodega e identificación por LPN según lo que informe Invas."
      frontend="frontend-web/src/app/(app)/recepciones/page.tsx"
      backend="backend-core/internal/modules/reception/"
      notes={["Nexus no genera ni reasigna LPN sin contrato explícito (DAS §4.4)."]}
    />
  );
}
