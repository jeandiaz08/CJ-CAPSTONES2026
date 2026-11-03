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
