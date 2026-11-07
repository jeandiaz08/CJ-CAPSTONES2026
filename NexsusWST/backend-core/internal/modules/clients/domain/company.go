// Package domain - empresas cliente de WeStorage (RF-04).
package domain

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("empresa no encontrada")

type Company struct {
	ID        int64
	Name      string
	TaxID     string // RUT
	WMSCode   string // vínculo con Invas
	Active    bool
	WaveCode  *string // ola por defecto del cliente (PV-19); nil = sin preferencia
	CreatedAt time.Time
}
