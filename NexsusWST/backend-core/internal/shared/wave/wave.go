// Package wave - código de ola de Invas que se guarda como preferencia en la
// ficha del cliente o del producto (PV-19). Mismo formato que las reglas.
package wave

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidCode = errors.New("código de ola inválido: usa letras, números, «-» o «_» (máximo 20)")
	reCode         = regexp.MustCompile(`^[A-Z0-9_-]{1,20}$`)
)

// Normalize limpia el código. Vacío = sin preferencia (nil).
func Normalize(code *string) (*string, error) {
	if code == nil {
		return nil, nil
	}
	c := strings.ToUpper(strings.TrimSpace(*code))
	if c == "" {
		return nil, nil
	}
	if !reCode.MatchString(c) {
		return nil, ErrInvalidCode
	}
	return &c, nil
}
