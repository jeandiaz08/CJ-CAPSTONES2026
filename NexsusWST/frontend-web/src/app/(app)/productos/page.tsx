"use client";

import { useEffect, useState } from "react";
import { Search } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { listProducts, type Product } from "@/services/core";
import { useAuth } from "@/state/auth";

// Catálogo de productos por cliente (RF-06).
// TODO (RF-06/07): crear/editar (services/core.ts ya tiene createProduct), kits, importar/exportar.
export default function ProductsPage() {
  const { companyId } = useAuth();
  const [items, setItems] = useState<Product[]>([]);
  const [q, setQ] = useState("");
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    listProducts(q, companyId)
      .then((r) => setItems(r.data))
      .catch((e) => setError(e.message));
  }, [q, companyId]);
  return (
    <>
      <PageHeader title="Productos" subtitle="Catálogo por empresa cliente" />
      <div className="filters card">
        <label className="search">
          <Search size={16} />
          <input placeholder="Buscar SKU o descripción" value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
      </div>
      {error && <p className="error">{error}</p>}
      <section className="card flush">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>SKU</th>
                <th>Descripción</th>
                <th>Tipo</th>
                <th>Empresa</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {items.map((p) => (
                <tr key={p.id}>
                  <td>{p.sku}</td>
                  <td>{p.description}</td>
                  <td>{p.type}</td>
                  <td>#{p.companyId}</td>
                  <td>{p.active ? "Activo" : "Inactivo"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}
