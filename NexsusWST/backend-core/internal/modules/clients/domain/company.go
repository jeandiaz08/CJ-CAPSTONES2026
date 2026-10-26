// Package domain - empresas cliente de WeStorage (RF-04).
package domain

import "time"

type Company struct {
	ID        int64
	Name      string
	TaxID     string // RUT
	WMSCode   string // vínculo con Invas
	Active    bool
	CreatedAt time.Time
}
