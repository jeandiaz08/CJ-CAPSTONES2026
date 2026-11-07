package domain

import (
	"errors"
	"time"
)

// Estrategia de etiqueta de cada courier (H-12 T09).
const (
	LabelStrategyAPI    = "api"
	LabelStrategyOffice = "office"
)

// Estado de la etiqueta de una orden. LabelOffice no se guarda: se deriva de la estrategia.
const (
	LabelNone    = "none"
	LabelPending = "pending"
	LabelReady   = "ready"
	LabelFailed  = "failed"
	LabelOffice  = "office"
)

// LabelMaxAttempts: con LabelRetryDelay son unos 45 minutos antes de marcarla como fallida.
const LabelMaxAttempts = 12

// ErrLabelNotReady: el courier todavía no entrega la etiqueta; se reintenta sola.
var ErrLabelNotReady = errors.New("el courier todavía no entrega la etiqueta")

// LabelRetryDelay es la espera creciente después de un intento.
func LabelRetryDelay(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return 30 * time.Second
	case attempt == 2:
		return time.Minute
	case attempt == 3:
		return 2 * time.Minute
	}
	return 5 * time.Minute
}

// LabelRetryDelayEvery (H-16-T06, SCRUM-245): la misma espera creciente, pero
// la espera estable («cada N minutos») es configurable. Con every <= 0 se usa
// LabelRetryDelay tal cual; si no, ningún intento espera más que every y,
// desde el cuarto, se reintenta cada every.
func LabelRetryDelayEvery(attempt int, every time.Duration) time.Duration {
	d := LabelRetryDelay(attempt)
	if every <= 0 {
		return d
	}
	if d > every || attempt >= 4 {
		return every
	}
	return d
}
