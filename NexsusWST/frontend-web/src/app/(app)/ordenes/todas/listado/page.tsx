import { Suspense } from "react";
import { OrdersListing } from "@/features/orders";
export default function Page() {
    return <Suspense fallback={<p>Cargando órdenes…</p>}>
    <OrdersListing />
    </Suspense>;
}
