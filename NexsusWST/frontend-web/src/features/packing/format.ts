import type { OrderItem } from "@/services/orders";

// «Ingresó hace…»: desde que la orden entró a Nexus. No cambia con las ediciones.
export function ageLabel(createdAt: string, now: number) {
  const minutes = Math.max(0, Math.floor((now - new Date(createdAt).getTime()) / 60000));
  if (minutes < 1) return "Ingresó hace menos de 1 min";
  if (minutes < 60) return `Ingresó hace ${minutes} min`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `Ingresó hace ${hours} h`;
  const days = Math.floor(hours / 24);
  return `Ingresó hace ${days} ${days === 1 ? "día" : "días"}`;
}

export const unitsOf = (items: OrderItem[] = []) => items.reduce((total, item) => total + item.quantity, 0);
