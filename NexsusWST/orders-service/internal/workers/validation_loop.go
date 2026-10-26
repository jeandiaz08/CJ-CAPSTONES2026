package workers

import (
	"context"
	"time"

	"github.com/nexuswst/orders-service/internal/application"
)

// RunValidationLoop barre periódicamente las órdenes Pendientes / En
// validación vencidas y las valida (RF-12, reintento con espera creciente).
func RunValidationLoop(ctx context.Context, v *application.Validator, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			v.RunOnce(ctx)
		}
	}
}

// RunEvery ejecuta fn periódicamente hasta que se cancela el contexto
// (motor de reglas de ola, sincronización de preparación con Invas).
func RunEvery(ctx context.Context, every time.Duration, fn func(context.Context)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn(ctx)
		}
	}
}
