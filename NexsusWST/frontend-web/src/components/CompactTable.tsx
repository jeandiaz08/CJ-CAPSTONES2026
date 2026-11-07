import type { ReactNode } from "react";

export type CompactColumn = { label: string; className?: string };
export type CompactRow = { key: string | number; cells: ReactNode[]; className?: string };

// Tabla compacta: encabezados arriba, una fila por elemento y desplazamiento horizontal
// (como «Información adicional» del detalle de una orden).
export function CompactTable({ label, columns, rows }: { label: string; columns: CompactColumn[]; rows: CompactRow[] }) {
  return (
    <div className="detail-scroll compact-table" role="region" aria-label={label} tabIndex={0}>
      <table>
        <thead><tr>{columns.map((c) => <th key={c.label} scope="col" className={c.className}>{c.label}</th>)}</tr></thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.key} className={row.className}>
              {row.cells.map((cell, i) => <td key={columns[i].label} className={columns[i].className}>{cell}</td>)}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
