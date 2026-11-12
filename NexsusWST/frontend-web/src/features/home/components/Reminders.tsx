"use client";

import { useCallback, useEffect, useState } from "react";
import { CirclePlus, NotebookPen, Trash2 } from "lucide-react";
import { showToast } from "@/components/Toast";
import { apiErrorMessage } from "@/services/api";
import { createNote, deleteNote, listNotes, NOTE_MAX_LENGTH, setNoteDone, type Note } from "@/services/notes";
import { dateInDays, dueLabel } from "../homeConfig";

const DUE_OPTIONS = [
  { label: "Sin fecha", days: null },
  { label: "Hoy", days: 0 },
  { label: "Mañana", days: 1 },
  { label: "En 3 días", days: 3 },
] as const;

// Recordatorios: notas personales del usuario (cada uno ve solo las suyas; lo filtra el Core).
export function Reminders() {
  const [notes, setNotes] = useState<Note[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [text, setText] = useState("");
  const [days, setDays] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);

  const load = useCallback(
    () => listNotes().then((r) => { setNotes(r.data); setError(null); }, (e) => setError(apiErrorMessage(e))),
    [],
  );
  useEffect(() => { void load(); }, [load]);

  // Tras cada cambio se vuelve a pedir la lista: el orden (pendientes primero, por fecha) es del servidor.
  const run = async (action: () => Promise<unknown>, done?: string) => {
    try {
      await action();
      if (done) showToast(done);
      await load();
      return true;
    } catch (e) {
      showToast(apiErrorMessage(e), "error");
      return false;
    }
  };

  const body = text.trim();
  const tooLong = [...body].length > NOTE_MAX_LENGTH;
  const closeForm = () => { setAdding(false); setText(""); setDays(null); };
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!body || tooLong || saving) return;
    setSaving(true);
    if (await run(() => createNote(body, days === null ? null : dateInDays(days)), "Nota agregada")) closeForm();
    setSaving(false);
  };

  return (
    <section className="home-reminders" aria-labelledby="reminders-title">
      <div>
        <h2 id="reminders-title" className="home-section-title">Recordatorios</h2>
        <p className="muted">Agrega notas y no olvides las tareas pendientes</p>
      </div>
      <div className="reminders-panel">
        <div className="reminders-head">
          <NotebookPen size={32} aria-hidden="true" />
          {!adding && <button type="button" className="btn reminders-new" onClick={() => setAdding(true)}><CirclePlus size={18} aria-hidden="true" />Nueva nota</button>}
        </div>

        {adding && (
          <form className="reminders-form" onSubmit={submit}>
            <label htmlFor="note-body">Nota</label>
            <input id="note-body" autoFocus value={text} onChange={(e) => setText(e.target.value)} placeholder="Escribe un recordatorio" aria-describedby="note-count" aria-invalid={tooLong} />
            <span id="note-count" className={`reminders-count ${tooLong ? "error" : ""}`} aria-live="polite">
              {[...body].length}/{NOTE_MAX_LENGTH}{tooLong && " · Máximo 140 caracteres"}
            </span>
            <fieldset>
              <legend>Fecha (opcional)</legend>
              {DUE_OPTIONS.map((o) => (
                <label key={o.label} className={`reminders-chip ${days === o.days ? "is-on" : ""}`}>
                  <input type="radio" name="note-due" checked={days === o.days} onChange={() => setDays(o.days)} />{o.label}
                </label>
              ))}
            </fieldset>
            <div className="actions">
              <button type="submit" className="btn primary" disabled={!body || tooLong || saving}>Guardar</button>
              <button type="button" className="btn ghost" onClick={closeForm}>Cancelar</button>
            </div>
          </form>
        )}

        {error ? (
          <p className="error" role="alert">No se pudieron cargar tus notas. {error} <button type="button" className="link-btn" onClick={() => void load()}>Reintentar</button></p>
        ) : notes === null ? (
          <p className="reminders-empty">Cargando…</p>
        ) : notes.length === 0 ? (
          <p className="reminders-empty">Aún no tienes notas. Agrega la primera con «Nueva nota».</p>
        ) : (
          <ul className="reminders-list">
            {notes.map((n) => (
              <li key={n.id} className={n.done ? "is-done" : ""}>
                <label>
                  <input type="checkbox" checked={n.done} onChange={() => void run(() => setNoteDone(n.id, !n.done))} />
                  <span>{n.body}</span>
                </label>
                <span className="reminders-due">{dueLabel(n.dueDate)}</span>
                <button type="button" className="icon-btn" aria-label={`Borrar nota: ${n.body}`} onClick={() => void run(() => deleteNote(n.id), "Nota borrada")}>
                  <Trash2 size={16} aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
