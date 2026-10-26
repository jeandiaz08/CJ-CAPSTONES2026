// Package wms - adaptadores hacia Invas (PA-01).
//
// SIMULADO: este adaptador NO llama a Invas. Está rotulado como simulación y
// no acredita una integración real (DAS §6.3). Cuando exista el contrato
// (PV-03/PV-04), crea invas.go que implemente ports.WMSGateway por HTTP.
//
// Reglas de la simulación (para probar los flujos):
//   - Stock: 100 unidades por SKU, salvo SKUs que contengan "SINSTOCK" (0)
//     o "POCO" (3).
//   - WMS_SIMULATE_DOWN=true hace fallar GetStock (prueba de reintentos).
//   - La preparación termina WMS_SIM_PICK_SECONDS después de confirmar la ola
//     (por defecto 60 s).
package wms

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

type Simulated struct{ log *slog.Logger }

func NewSimulated(log *slog.Logger) *Simulated {
	return &Simulated{log: log.With("adapter", "wms-simulado")}
}

func (s *Simulated) GetStock(ctx context.Context, companyID int64, skus []string) (map[string]int, error) {
	if os.Getenv("WMS_SIMULATE_DOWN") == "true" {
		return nil, errors.New("[SIMULADO] Invas no responde")
	}
	out := make(map[string]int, len(skus))
	for _, sku := range skus {
		switch {
		case strings.Contains(sku, "SINSTOCK"):
			out[sku] = 0
		case strings.Contains(sku, "POCO"):
			out[sku] = 3
		default:
			out[sku] = 100
		}
	}
	return out, nil
}

func (s *Simulated) ListWaves(ctx context.Context) ([]ports.Wave, error) {
	if os.Getenv("WMS_SIMULATE_DOWN") == "true" {
		return nil, errors.New("[SIMULADO] Invas no responde")
	}
	day := time.Now().Format("0201")
	return []ports.Wave{
		{ID: "OLA-" + day + "-GEN", Code: "GEN", Name: "Ola general " + day, Type: "general"},
		{ID: "OLA-" + day + "-AM", Code: "AM", Name: "Ola mañana " + day, Type: "automatica"},
		{ID: "OLA-" + day + "-PM", Code: "PM", Name: "Ola tarde " + day, Type: "automatica"},
	}, nil
}

func (s *Simulated) AssignToWave(ctx context.Context, o domain.Order, waveID string) (string, error) {
	if os.Getenv("WMS_SIMULATE_DOWN") == "true" {
		return "", errors.New("[SIMULADO] Invas no responde")
	}
	select {
	case <-time.After(500 * time.Millisecond): // Invas real: 4–6 min por asignación
	case <-ctx.Done():
		return "", ctx.Err()
	}
	ref := fmt.Sprintf("SIM-OS-%d", o.ID)
	s.log.Info("[SIMULADO] orden asignada a ola", "order_id", o.ID, "wave", waveID, "os", ref)
	return ref, nil
}

func (s *Simulated) PickingDone(ctx context.Context, o domain.Order) (bool, error) {
	secs := 60
	if v, err := strconv.Atoi(os.Getenv("WMS_SIM_PICK_SECONDS")); err == nil && v >= 0 {
		secs = v
	}
	return time.Since(o.UpdatedAt) >= time.Duration(secs)*time.Second, nil
}
