// Package domain - catálogos de baja variabilidad (RF-29/30/31).
package domain

import "errors"

var ErrUnknownCatalog = errors.New("catálogo desconocido")

// Name identifica un catálogo cacheable. Agrega aquí uno nuevo (bodegas,
// tipos de despacho, comunas...) y su consulta en el adaptador PostgreSQL.
type Name string

const (
	Regions       Name = "regions"
	Couriers      Name = "couriers"
	Communes      Name = "communes"       // destinos (H-07) y cobertura de couriers
	DispatchTypes Name = "dispatch_types" // tipos de despacho (reglas de ola, RF-17)
)

func Known(n Name) bool {
	switch n {
	case Regions, Couriers, Communes, DispatchTypes:
		return true
	}
	return false
}
