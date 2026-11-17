"use client";

import { useParams } from "next/navigation";
import { OrderRedirect } from "@/features/orders";

// Enlaces antiguos a una orden: abren su detalle en Todas las órdenes (o su registro).
export default function Page() {
  const { id } = useParams<{ id: string }>();
  return <OrderRedirect id={Number(id)} />;
}
