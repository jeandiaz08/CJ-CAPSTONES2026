// Recordatorios del Home: notas personales (Core, /v1/me/notes). El servidor las filtra por el
// usuario del token: cada persona ve y cambia solo las suyas.
import { api } from "./api";

export const NOTE_MAX_LENGTH = 140;

export type Note = {
  id: number;
  body: string;
  dueDate: string | null; // AAAA-MM-DD
  done: boolean;
  doneAt: string | null;
  createdAt: string;
  updatedAt: string;
};

export const listNotes = () => api<{ data: Note[] }>("core", "/v1/me/notes");
export const createNote = (body: string, dueDate: string | null) =>
  api<Note>("core", "/v1/me/notes", { method: "POST", body: JSON.stringify({ body, dueDate }) });
export const setNoteDone = (id: number, done: boolean) =>
  api<Note>("core", `/v1/me/notes/${id}`, { method: "PATCH", body: JSON.stringify({ done }) });
export const deleteNote = (id: number) => api<void>("core", `/v1/me/notes/${id}`, { method: "DELETE" });
