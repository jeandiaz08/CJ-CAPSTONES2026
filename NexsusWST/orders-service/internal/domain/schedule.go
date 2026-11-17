package domain

import (
	"slices"
	"time"
	_ "time/tzdata" // la imagen de producción no trae zonas horarias del sistema
)

var santiago = func() *time.Location {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		panic(err)
	}
	return loc
}()

// WeekDays en el formato de operation_days; el índice coincide con time.Weekday.
var WeekDays = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// OperationSchedule: horario de operación de ops_settings. Las horas son HH:MM de
// America/Santiago, por eso se pueden comparar como texto.
type OperationSchedule struct {
	AssignmentDeadline        string   `json:"assignmentDeadline"`
	OperationStart            string   `json:"operationStart"`
	OperationDays             []string `json:"operationDays"`
	PriorityOverridesDeadline bool     `json:"priorityOverridesDeadline"`
}

// QueueDecision: qué hace el motor con una orden en este momento.
type QueueDecision struct {
	Assign      bool
	Rank        string    // corte que define su lugar en la cola; ver QueueRank
	OutOfCutoff bool      // retenida por haber pasado el último corte de su courier
	ReleaseAt   time.Time // si se retiene, cuándo vuelve a evaluarse
}

// PriorityRank adelanta a una prioritaria fuera de corte: pidieron que salga hoy.
const PriorityRank = "00:00"

func hourMinute(t time.Time) string { return t.In(santiago).Format("15:04") }

func (s OperationSchedule) IsOperationDay(now time.Time) bool {
	return slices.Contains(s.OperationDays, WeekDays[now.In(santiago).Weekday()])
}

func (s OperationSchedule) BeforeStart(now time.Time) bool {
	return hourMinute(now) < s.OperationStart
}

func (s OperationSchedule) AfterDeadline(now time.Time) bool {
	return s.AssignmentDeadline != "" && hourMinute(now) > s.AssignmentDeadline
}

// NextRelease: el próximo día de operación a la hora de inicio.
func (s OperationSchedule) NextRelease(now time.Time) time.Time {
	local := now.In(santiago)
	var hour, minute int
	if t, err := time.Parse("15:04", s.OperationStart); err == nil {
		hour, minute = t.Hour(), t.Minute()
	}
	for days := 0; days <= 7; days++ {
		day := local.AddDate(0, 0, days)
		start := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, santiago)
		if s.IsOperationDay(start) && start.After(local) {
			return start
		}
	}
	return time.Time{} // sin días de operación configurados
}

// nextCutoff: el corte más temprano que todavía no pasa; un corte vale hasta su minuto exacto.
func nextCutoff(now time.Time, cutoffs []string) string {
	current, next := hourMinute(now), ""
	for _, cutoff := range cutoffs {
		if cutoff >= current && (next == "" || cutoff < next) {
			next = cutoff
		}
	}
	return next
}

// QueueRank ordena la cola: el corte más temprano primero. Una prioritaria que ya pasó
// sus cortes va adelante; el resto sin próximo corte queda vacío y va al final.
func QueueRank(now time.Time, courierCutoffs []string, priority bool) string {
	next := nextCutoff(now, courierCutoffs)
	if next == "" && priority && len(courierCutoffs) > 0 {
		return PriorityRank
	}
	return next
}

// Decide aplica las reglas de cortes del Jefe de Operaciones al motor automático. Se
// refleja en SQL en assignmentRetained: un cambio aquí también debe hacerse allá.
func (s OperationSchedule) Decide(now time.Time, courierCutoffs []string, priority bool) QueueDecision {
	if !s.IsOperationDay(now) || s.BeforeStart(now) {
		return QueueDecision{ReleaseAt: s.NextRelease(now)}
	}
	if s.AfterDeadline(now) && !(priority && s.PriorityOverridesDeadline) {
		return QueueDecision{ReleaseAt: s.NextRelease(now)}
	}
	// Un courier sin cortes activos no queda sin salir por un olvido de configuración.
	if len(courierCutoffs) > 0 && nextCutoff(now, courierCutoffs) == "" && !priority {
		return QueueDecision{OutOfCutoff: true, ReleaseAt: s.NextRelease(now)}
	}
	return QueueDecision{Assign: true, Rank: QueueRank(now, courierCutoffs, priority)}
}

// QueueLess: corte → prioridad → antigüedad. Sin corte (Rank vacío) va al final.
func QueueLess(rankA string, priorityA bool, createdA time.Time, idA int64, rankB string, priorityB bool, createdB time.Time, idB int64) bool {
	if rankA != rankB {
		if rankA == "" || rankB == "" {
			return rankB == ""
		}
		return rankA < rankB
	}
	if priorityA != priorityB {
		return priorityA
	}
	if !createdA.Equal(createdB) {
		return createdA.Before(createdB)
	}
	return idA < idB
}
