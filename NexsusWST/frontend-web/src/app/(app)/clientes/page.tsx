"use client";

import { useEffect, useState } from "react";
import { PageHeader } from "@/components/PageHeader";
import { listCompanies, type Company } from "@/services/core";

// Empresas cliente (RF-04). Hoy solo lista.
// TODO (RF-04/05): crear, editar, activar/desactivar e integraciones por cliente.
//   Backend: backend-core/internal/modules/clients/
export default function ClientsPage() {
  const [items, setItems] = useState<Company[]>([]);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    listCompanies()
      .then((r) => setItems(r.data))
      .catch((e) => setError(e.message));
  }, []);
  return (
    <>
      <PageHeader title="Clientes" subtitle="Empresas cliente de WeStorage" />
      {error && <p className="error">{error}</p>}
      <section className="card flush">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Nombre</th>
                <th>RUT</th>
                <th>Código WMS</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {items.map((c) => (
                <tr key={c.id}>
                  <td>{c.id}</td>
                  <td>{c.name}</td>
                  <td>{c.taxId}</td>
                  <td>{c.wmsCode}</td>
                  <td>{c.active ? "Activo" : "Inactivo"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}
