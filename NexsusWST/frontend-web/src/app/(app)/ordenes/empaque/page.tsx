import { Suspense } from "react";
import { PackingQueue } from "@/features/packing";

export default function Page() {
  return <Suspense fallback={<p>Cargando empaque…</p>}><PackingQueue /></Suspense>;
}
