import { Suspense } from "react";
import { ProductsView } from "@/features/products";

// Productos (Tarea 09, H-04).
export default function Page() {
  return (
    <Suspense fallback={<p className="muted">Cargando…</p>}>
      <ProductsView />
    </Suspense>
  );
}
