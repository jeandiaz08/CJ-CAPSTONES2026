package domain

import (
	"testing"
	"time"
)

func TestLabelRetryDelayGrows(t *testing.T) {
	want := map[int]time.Duration{1: 30 * time.Second, 2: time.Minute, 3: 2 * time.Minute, 4: 5 * time.Minute, 11: 5 * time.Minute}
	for attempt, delay := range want {
		if got := LabelRetryDelay(attempt); got != delay {
			t.Fatalf("intento %d: %v, esperado %v", attempt, got, delay)
		}
	}
}

func TestLabelRetryDelayEveryIsConfigurable(t *testing.T) {
	// Sin configurar: igual que antes.
	for attempt := 1; attempt <= 6; attempt++ {
		if LabelRetryDelayEvery(attempt, 0) != LabelRetryDelay(attempt) {
			t.Fatalf("intento %d cambió sin configuración", attempt)
		}
	}
	// Cada 10 minutos: crece igual y luego se estabiliza en 10.
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 10 * time.Minute, 10 * time.Minute}
	for i, w := range want {
		if got := LabelRetryDelayEvery(i+1, 10*time.Minute); got != w {
			t.Fatalf("intento %d: %v, quería %v", i+1, got, w)
		}
	}
	// Cada 1 minuto: nunca espera más de 1 minuto.
	for attempt := 1; attempt <= 6; attempt++ {
		if got := LabelRetryDelayEvery(attempt, time.Minute); got > time.Minute {
			t.Fatalf("intento %d esperó %v", attempt, got)
		}
	}
}
