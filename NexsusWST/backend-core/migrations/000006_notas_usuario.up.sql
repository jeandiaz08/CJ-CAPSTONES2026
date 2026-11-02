-- backend-core · notas personales del Home («Recordatorios»)
-- Cada nota pertenece a un usuario y solo la ve él. Texto de hasta 140 caracteres,
-- fecha opcional («Hoy», «Mañana», «En 3 días» se calculan desde due_date) y check.

CREATE TABLE user_notes (
    id         bigserial PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       text NOT NULL CHECK (char_length(btrim(body)) BETWEEN 1 AND 140),
    due_date   date,
    done       boolean NOT NULL DEFAULT false,
    done_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_notes_done_at_check CHECK (done = (done_at IS NOT NULL))
);

CREATE INDEX idx_user_notes_user ON user_notes (user_id, done, due_date);
