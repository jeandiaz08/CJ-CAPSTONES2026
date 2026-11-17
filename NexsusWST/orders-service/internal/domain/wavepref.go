package domain

import "fmt"

// Ola por producto o por cliente (PV-19, H-10-T11). Va ANTES de las reglas de
// asignación: producto → cliente → reglas → sin regla que calce.
//
//   - Si los productos de la orden que tienen ola propia coinciden en una
//     sola ola, la orden va a esa ola.
//   - Si tienen olas distintas entre sí, o ninguno tiene, se usa la ola por
//     defecto del cliente.
//   - Si tampoco hay ola del cliente, decide el motor de reglas.
//
// Los cortes y el horario no cambian: deciden CUÁNDO sale la orden; esto
// decide A QUÉ ola va.

type WaveSource string

const (
	WaveFromProduct WaveSource = "producto"
	WaveFromCompany WaveSource = "cliente"
)

// WavePreference es la ola elegida antes de las reglas y por qué.
type WavePreference struct {
	Code   string
	Source WaveSource
	Reason string // texto para el historial de la orden
}

// ResolveWavePreference aplica producto → cliente. productWaves son las olas
// DISTINTAS de los productos de la orden que tienen una; companyWave es la ola
// por defecto del cliente ("" si no tiene). ok=false: deciden las reglas.
func ResolveWavePreference(productWaves []string, companyWave string) (WavePreference, bool) {
	if len(productWaves) == 1 {
		return WavePreference{Code: productWaves[0], Source: WaveFromProduct,
			Reason: "Ola del producto → " + productWaves[0]}, true
	}
	if companyWave != "" {
		reason := "Ola por defecto del cliente → " + companyWave
		if len(productWaves) > 1 {
			reason = fmt.Sprintf("Los productos tienen olas distintas (%v): se usa la ola del cliente → %s", productWaves, companyWave)
		}
		return WavePreference{Code: companyWave, Source: WaveFromCompany, Reason: reason}, true
	}
	return WavePreference{}, false
}
