import { PageHeader } from "./PageHeader";

// Pantalla base de un módulo aún no implementado: indica qué cubre y dónde programarlo.
export function ModulePlaceholder({
  title,
  rf,
  description,
  frontend,
  backend,
  notes,
}: {
  title: string;
  rf?: string;
  description: string;
  frontend: string;
  backend: string;
  notes?: string[];
}) {
  return (
    <>
      <PageHeader title={title} subtitle={rf ? `Requisitos: ${rf}` : undefined} />
      <section className="card placeholder">
        <p>{description}</p>
        <h3>Dónde programarlo</h3>
        <ul className="paths">
          <li>
            <span className="muted">Pantalla</span> <code>{frontend}</code>
          </li>
          <li>
            <span className="muted">Backend</span> <code>{backend}</code>
          </li>
        </ul>
        {notes && notes.length > 0 && (
          <>
            <h3>A tener en cuenta</h3>
            <ul>
              {notes.map((n) => (
                <li key={n}>{n}</li>
              ))}
            </ul>
          </>
        )}
      </section>
    </>
  );
}
