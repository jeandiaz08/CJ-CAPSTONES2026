// Package domain - empresas cliente de WeStorage (RF-04, H-03).
package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexuswst/backend-core/internal/shared/wave"
)

var (
	ErrNotFound    = errors.New("empresa no encontrada")
	ErrTaxIDTaken  = errors.New("ya existe una empresa con ese RUT")
	ErrSameState   = errors.New("la empresa ya está en ese estado")
	ErrReasonShort = errors.New("indica el motivo (entre 5 y 500 caracteres)")
)

type Company struct {
	ID            int64
	Name          string
	TaxID         string // RUT (obligatorio y único desde Core 000011)
	WMSCode       string // vínculo con Invas (opcional)
	Email         string // contacto (opcional)
	Phone         string // contacto (opcional)
	Active        bool
	WaveCode      *string // ola por defecto del cliente (PV-19); nil = «General (según reglas)»
	ProfilesCount int     // perfiles con rol Cliente asociados
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Draft son los datos que se guardan al crear o editar una empresa.
type Draft struct {
	Name     string
	TaxID    string
	Email    string
	Phone    string
	WaveCode *string
	WMSCode  string
}

// FieldError y ValidationErrors: el 422 indica el motivo por campo (el formulario los muestra
// junto a cada campo, no aplanados).
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors []FieldError

func (v ValidationErrors) Error() string {
	msgs := make([]string, 0, len(v))
	for _, f := range v {
		msgs = append(msgs, f.Message)
	}
	return strings.Join(msgs, " · ")
}

// Normalize limpia el borrador y valida lo que exige la base de datos (NOT NULL, formato de la ola,
// largo del teléfono) más el dígito verificador del RUT. checkRUT es false al editar sin cambiar el
// RUT, para no bloquear la edición de empresas antiguas con un RUT de otro formato.
func (d *Draft) Normalize(checkRUT bool) ValidationErrors {
	var errs ValidationErrors
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }

	d.Name = strings.Join(strings.Fields(d.Name), " ")
	d.Email = strings.ToLower(strings.TrimSpace(d.Email))
	d.Phone = strings.TrimSpace(d.Phone)
	d.WMSCode = strings.TrimSpace(d.WMSCode)
	d.TaxID = strings.TrimSpace(d.TaxID)

	if d.Name == "" || utf8.RuneCountInString(d.Name) > 120 {
		add("name", "El nombre es obligatorio (máximo 120 caracteres).")
	}
	switch {
	case d.TaxID == "":
		add("taxId", "El RUT es obligatorio.")
	case checkRUT:
		if rut, ok := NormalizeRUT(d.TaxID); ok {
			d.TaxID = rut
		} else {
			add("taxId", "RUT inválido: revisa el número y el dígito verificador (ej.: 76.123.456-0).")
		}
	}
	if d.Email != "" {
		if a, err := mail.ParseAddress(d.Email); err != nil || a.Address != d.Email || !strings.Contains(d.Email[strings.Index(d.Email, "@"):], ".") {
			add("email", "Correo inválido.")
		}
	}
	if d.Phone != "" && (len(d.Phone) < 6 || len(d.Phone) > 30) {
		add("phone", "Teléfono inválido (entre 6 y 30 caracteres).")
	}
	if utf8.RuneCountInString(d.WMSCode) > 40 {
		add("wmsCode", "El código Invas admite hasta 40 caracteres.")
	}
	code, err := wave.Normalize(d.WaveCode)
	if err != nil {
		add("waveCode", "Ola inválida: usa el código de la ola de Invas o «General (según reglas)».")
	}
	d.WaveCode = code
	return errs
}

// NormalizeReason valida el motivo de activar o desactivar.
func NormalizeReason(reason string, required bool) (string, error) {
	r := strings.TrimSpace(reason)
	n := utf8.RuneCountInString(r)
	if (required && n < 5) || n > 500 {
		return "", ErrReasonShort
	}
	return r, nil
}
