package domain

import (
	"testing"
	"time"
)

var weekdays = OperationSchedule{AssignmentDeadline: "12:00", OperationStart: "06:30", OperationDays: []string{"mon", "tue", "wed", "thu", "fri"}}

// Semana de noviembre de 2026: lunes 2 a domingo 8; lunes siguiente, 9.
func at(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, santiago)
}

func TestDecide(t *testing.T) {
	cutoffs := []string{"08:00", "10:00", "11:00"}
	nextMonday := at(9, 6, 30)
	for _, tc := range []struct {
		name       string
		now        time.Time
		cutoffs    []string
		priority   bool
		assign     bool
		rank       string
		outOfCut   bool
		releaseAt  time.Time
		overriding bool
	}{
		{name: "antes del primer corte", now: at(2, 7, 0), cutoffs: cutoffs, assign: true, rank: "08:00"},
		{name: "un corte vale hasta su minuto exacto", now: at(2, 8, 0), cutoffs: cutoffs, assign: true, rank: "08:00"},
		{name: "pasa al próximo corte", now: at(2, 8, 1), cutoffs: cutoffs, assign: true, rank: "10:00"},
		{name: "creada 10:55 y validada 11:05 con corte 11:00: retenida", now: at(2, 11, 5), cutoffs: []string{"11:00"}, outOfCut: true, releaseAt: at(3, 6, 30)},
		{name: "prioritaria fuera de corte sale el mismo día, adelante", now: at(2, 11, 5), cutoffs: []string{"11:00"}, priority: true, assign: true, rank: PriorityRank},
		{name: "prioritaria en hora normal respeta su corte", now: at(2, 9, 0), cutoffs: cutoffs, priority: true, assign: true, rank: "10:00"},
		{name: "viernes después de la hora límite: lunes 06:30", now: at(6, 12, 1), cutoffs: cutoffs, releaseAt: nextMonday},
		{name: "la hora límite vale hasta su minuto exacto", now: at(6, 12, 0), cutoffs: []string{"12:00"}, assign: true, rank: "12:00"},
		{name: "sábado: lunes", now: at(7, 9, 0), cutoffs: cutoffs, releaseAt: nextMonday},
		{name: "domingo: lunes", now: at(8, 23, 0), cutoffs: cutoffs, releaseAt: nextMonday},
		{name: "lunes antes del inicio: hoy 06:30", now: at(9, 6, 29), cutoffs: cutoffs, releaseAt: nextMonday},
		{name: "lunes a la hora de inicio: se libera", now: nextMonday, cutoffs: cutoffs, assign: true, rank: "08:00"},
		{name: "la prioridad no salta la hora límite", now: at(2, 13, 0), cutoffs: cutoffs, priority: true, releaseAt: at(3, 6, 30)},
		{name: "con el parámetro, la prioridad salta la hora límite", now: at(2, 13, 0), cutoffs: cutoffs, priority: true, overriding: true, assign: true, rank: PriorityRank},
		{name: "con el parámetro, las normales no la saltan", now: at(2, 13, 0), cutoffs: cutoffs, overriding: true, releaseAt: at(3, 6, 30)},
		{name: "la prioridad no asigna en días sin operación", now: at(7, 9, 0), cutoffs: cutoffs, priority: true, overriding: true, releaseAt: nextMonday},
		{name: "courier sin cortes se asigna al final", now: at(2, 11, 30), assign: true, rank: ""},
		{name: "courier sin cortes respeta la hora límite", now: at(2, 12, 30), releaseAt: at(3, 6, 30)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := weekdays
			s.PriorityOverridesDeadline = tc.overriding
			got := s.Decide(tc.now, tc.cutoffs, tc.priority)
			if got.Assign != tc.assign || got.Rank != tc.rank || got.OutOfCutoff != tc.outOfCut {
				t.Fatalf("decisión: %+v", got)
			}
			if !got.ReleaseAt.Equal(tc.releaseAt) {
				t.Fatalf("se libera %v, esperado %v", got.ReleaseAt, tc.releaseAt)
			}
		})
	}
}

func TestDecideUsesSantiagoTime(t *testing.T) {
	utc := time.Date(2026, time.November, 2, 10, 0, 0, 0, time.UTC) // 07:00 en Santiago
	if got := weekdays.Decide(utc, []string{"08:00"}, false); !got.Assign || got.Rank != "08:00" {
		t.Fatalf("debe evaluar en hora de Santiago: %+v", got)
	}
}

func TestNextReleaseWithoutOperationDays(t *testing.T) {
	s := OperationSchedule{OperationStart: "06:30"}
	if got := s.NextRelease(at(2, 9, 0)); !got.IsZero() {
		t.Fatalf("sin días de operación no hay próxima liberación: %v", got)
	}
}

func TestQueueLess(t *testing.T) {
	old, recent := at(6, 9, 0), at(7, 9, 0)
	for _, tc := range []struct {
		name  string
		less  bool
		rankA string
		prioA bool
		timeA time.Time
		rankB string
		prioB bool
		timeB time.Time
	}{
		{"el corte manda sobre la prioridad", true, "08:00", false, recent, "11:00", true, old},
		{"mismo corte: la prioridad va primero", true, "08:00", true, recent, "08:00", false, old},
		{"mismo corte y prioridad: lo más antiguo primero", true, "08:00", false, old, "08:00", false, recent},
		{"sin corte va al final", true, "11:00", false, recent, "", true, old},
		{"prioritaria fuera de corte adelante", true, PriorityRank, true, recent, "08:00", true, old},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueueLess(tc.rankA, tc.prioA, tc.timeA, 1, tc.rankB, tc.prioB, tc.timeB, 2); got != tc.less {
				t.Fatalf("A antes que B: %v", got)
			}
			if got := QueueLess(tc.rankB, tc.prioB, tc.timeB, 2, tc.rankA, tc.prioA, tc.timeA, 1); got == tc.less {
				t.Fatalf("B antes que A: %v", got)
			}
		})
	}
}
