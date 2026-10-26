"use client";

import { useEffect, useState } from "react";
import { Plus, X } from "lucide-react";
import { Modal } from "@/components/Modal";
import { getCatalog, listCompanies, type CatalogItem, type Company } from "@/services/core";
import { createOrder, type OrderItem } from "@/services/orders";
import { useAuth } from "@/state/auth";

// Ingreso manual de una orden (RF-08). La validación es automática al guardar
// (productos, comuna, cobertura y stock). Carga masiva y documentos: ver
// POST /v1/orders/import y /v1/orders/{id}/documents en orders-service/API.md.
export function CreateOrderDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const { companyId } = useAuth();
  const [companies, setCompanies] = useState<Company[]>([]);
  const [couriers, setCouriers] = useState<CatalogItem[]>([]);
  const [communes, setCommunes] = useState<{ name: string }[]>([]);
  const [form, setForm] = useState({
    companyId: companyId ?? 0,
    externalRef: "",
    channel: "manual",
    courier: "",
    recipientName: "",
    recipientPhone: "",
    addressLine: "",
    commune: "",
    dispatchType: "domicilio",
    customerNotes: "",
  });
  const [items, setItems] = useState<OrderItem[]>([{ sku: "", quantity: 1 }]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    listCompanies()
      .then((r) => {
        setCompanies(r.data);
        if (!form.companyId && r.data[0]) setForm((f) => ({ ...f, companyId: r.data[0].id }));
      })
      .catch(() => undefined);
    getCatalog("couriers")
      .then((r) => setCouriers(r.data))
      .catch(() => undefined);
    getCatalog("communes")
      .then((r) => setCommunes(r.data as unknown as { name: string }[]))
      .catch(() => undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await createOrder({ ...form, items: items.filter((i) => i.sku.trim()) });
      onCreated();
      onClose();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="Nueva orden manual" onClose={onClose}>
      <form onSubmit={submit} className="form">
        <label>
          Empresa
          <select value={form.companyId} onChange={(e) => setForm({ ...form, companyId: Number(e.target.value) })}>
            {companies.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        <div className="grid2">
          <label>
            Referencia (opcional)
            <input value={form.externalRef} onChange={(e) => setForm({ ...form, externalRef: e.target.value })} placeholder="Se genera si la dejas vacía" />
          </label>
          <label>
            Canal
            <select value={form.channel} onChange={(e) => setForm({ ...form, channel: e.target.value })}>
              <option value="manual">Manual</option>
              <option value="mercadolibre">Mercado Libre</option>
              <option value="shopify">Shopify</option>
              <option value="web">Web</option>
            </select>
          </label>
        </div>
        <label>
          Courier
          <select required value={form.courier} onChange={(e) => setForm({ ...form, courier: e.target.value })}>
            <option value="">Elige un courier</option>
            {couriers.map((c) => (
              <option key={c.code} value={c.code}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        <fieldset>
          <legend>Destino</legend>
          <div className="grid2">
            <label>
              Destinatario
              <input required value={form.recipientName} onChange={(e) => setForm({ ...form, recipientName: e.target.value })} />
            </label>
            <label>
              Teléfono
              <input value={form.recipientPhone} onChange={(e) => setForm({ ...form, recipientPhone: e.target.value })} placeholder="+56 9 ..." />
            </label>
          </div>
          <label>
            Dirección (calle y número)
            <input required value={form.addressLine} onChange={(e) => setForm({ ...form, addressLine: e.target.value })} />
          </label>
          <div className="grid2">
            <label>
              Comuna
              <input required list="communes" value={form.commune} onChange={(e) => setForm({ ...form, commune: e.target.value })} />
              <datalist id="communes">
                {communes.map((c) => (
                  <option key={c.name} value={c.name} />
                ))}
              </datalist>
            </label>
            <label>
              Tipo de despacho
              <select value={form.dispatchType} onChange={(e) => setForm({ ...form, dispatchType: e.target.value })}>
                <option value="domicilio">Despacho a domicilio</option>
                <option value="retiro_oficina">Retiro en oficina del courier</option>
                <option value="express">Express (mismo día)</option>
              </select>
            </label>
          </div>
        </fieldset>
        <fieldset>
          <legend>Productos</legend>
          {items.map((it, i) => (
            <div key={i} className="item-row">
              <input
                required
                placeholder="SKU (ej. SKU-001)"
                value={it.sku}
                onChange={(e) => setItems(items.map((x, j) => (j === i ? { ...x, sku: e.target.value } : x)))}
              />
              <input
                type="number"
                min={1}
                value={it.quantity}
                onChange={(e) => setItems(items.map((x, j) => (j === i ? { ...x, quantity: Number(e.target.value) } : x)))}
              />
              {items.length > 1 && (
                <button type="button" className="icon-btn" onClick={() => setItems(items.filter((_, j) => j !== i))} aria-label="Quitar">
                  <X size={16} />
                </button>
              )}
            </div>
          ))}
          <button type="button" className="btn ghost" onClick={() => setItems([...items, { sku: "", quantity: 1 }])}>
            <Plus size={15} /> Agregar producto
          </button>
        </fieldset>
        {error && <p className="error">{error}</p>}
        <div className="form-actions">
          <button type="button" className="btn ghost" onClick={onClose}>
            Cancelar
          </button>
          <button className="btn primary" disabled={busy}>
            {busy ? "Guardando…" : "Crear orden"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
