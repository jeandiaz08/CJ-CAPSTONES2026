"use client";

import { useEffect, useState } from "react";
import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { OrdersTable } from "@/features/orders/OrdersTable";
import { useOrders } from "@/hooks/useOrders";
import { assignWave, listWaves, type Wave } from "@/services/orders";

// Asignación MANUAL de órdenes Validadas a olas de Invas (RF-17, CU-02).
// Asignar = encolar: el Worker envía a Invas en lotes pequeños, primero las prioritarias.
// TODO (RF-17): asignación automática por reglas (ver /reglas-olas).
export default function WavesPage() {
  const { orders, loading, error, reload, live } = useOrders({ status: "validated", withoutWave: true, limit: 200 });
  const [waves, setWaves] = useState<Wave[]>([]);
  const [waveId, setWaveId] = useState("");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    listWaves()
      .then((r) => {
        setWaves(r.data);
        setWaveId(r.data[0]?.id ?? "");
      })
      .catch((e) => setMsg(e.message));
  }, []);

  const toggle = (id: number) => {
    const s = new Set(selected);
    if (s.has(id)) s.delete(id);
    else s.add(id);
    setSelected(s);
  };

  const assign = async () => {
    try {
      const r = await assignWave([...selected], waveId);
      setMsg(`${r.queued.length} orden(es) encoladas para la ola ${waveId}. Invas confirmará la asignación.`);
      setSelected(new Set());
      reload();
    } catch (e) {
      setMsg((e as Error).message);
    }
  };

  return (
    <>
      <PageHeader title="Asignación a olas" subtitle="Órdenes Validadas sin ola" actions={<LiveBadge live={live} />} />
      <div className="filters card">
        <label>
          Ola de Invas{" "}
          <select value={waveId} onChange={(e) => setWaveId(e.target.value)}>
            {waves.map((w) => (
              <option key={w.id} value={w.id}>
                {w.name} ({w.type})
              </option>
            ))}
          </select>
        </label>
        <button className="btn ghost" onClick={() => setSelected(new Set(orders.map((o) => o.id)))}>
          Seleccionar todas
        </button>
        <button className="btn primary" disabled={selected.size === 0 || !waveId} onClick={assign}>
          Asignar {selected.size || ""} a la ola
        </button>
      </div>
      {msg && <p className="callout">{msg}</p>}
      {error && <p className="error">{error}</p>}
      <section className="card flush">
        <OrdersTable orders={orders} loading={loading} selectable selected={selected} onToggle={toggle} onChanged={reload} empty="No hay órdenes Validadas esperando ola." />
      </section>
    </>
  );
}
