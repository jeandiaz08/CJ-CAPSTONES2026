package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Catálogo de productos (Tarea 09, H-04): reglas del formulario «Nuevo producto» y «Editar
// producto». El SKU es único por empresa y, como el cliente y el tipo, no cambia después de crear
// el producto (las órdenes y el WMS usan el SKU). Un producto no se borra: se desactiva con motivo.

const (
	MaxSKULength         = 60
	MaxDescriptionLength = 200
)

var (
	// ErrSameState: el producto ya está en ese estado.
	ErrSameState = errors.New("el producto ya está en ese estado; recarga la página")
	// ErrReasonRequired: desactivar exige motivo.
	ErrReasonRequired = errors.New("indica el motivo (entre 5 y 500 caracteres)")
)

// FieldError y ValidationErrors: el 422 indica el motivo por campo.
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

// InUseError: no se puede desactivar un producto que es componente de kits activos (409).
type InUseError struct{ Kits []string }

func (e InUseError) Error() string {
	return "el producto es componente de kits activos (" + strings.Join(e.Kits, ", ") + "): desactiva o edita esos kits primero"
}

// InactiveComponentsError: no se activa un kit con componentes inactivos (409).
type InactiveComponentsError struct{ SKUs []string }

func (e InactiveComponentsError) Error() string {
	return "el kit tiene componentes inactivos (" + strings.Join(e.SKUs, ", ") + "): actívalos primero"
}

// ProductDraft son los datos del formulario o de una fila de la importación.
type ProductDraft struct {
	CompanyID   int64
	SKU         string
	Description string
	Type        ProductType
	WaveCode    *string
}

// NormalizeSKU: sin espacios a los lados y en mayúsculas (igual que NewProduct).
func NormalizeSKU(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// Normalize valida cliente, SKU, descripción y tipo (withKeys: false al editar, porque cliente,
// SKU y tipo no cambian). La ola la valida wave.Normalize.
func (d *ProductDraft) Normalize(withKeys bool) ValidationErrors {
	var errs ValidationErrors
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }
	d.SKU = NormalizeSKU(d.SKU)
	d.Description = strings.Join(strings.Fields(d.Description), " ")
	if withKeys {
		if d.CompanyID <= 0 {
			add("companyId", "Elige el cliente.")
		}
		switch {
		case d.SKU == "":
			add("sku", "El SKU es obligatorio.")
		case utf8.RuneCountInString(d.SKU) > MaxSKULength:
			add("sku", "El SKU admite hasta 60 caracteres.")
		case strings.ContainsAny(d.SKU, " \t"):
			add("sku", "El SKU no puede tener espacios.")
		}
		if d.Type == "" {
			d.Type = TypeSimple
		}
		if d.Type != TypeSimple && d.Type != TypeKit {
			add("type", "Elige el tipo: Artículo o Kit.")
		}
	}
	if d.Description == "" || utf8.RuneCountInString(d.Description) > MaxDescriptionLength {
		add("description", "La descripción es obligatoria (máximo 200 caracteres).")
	}
	return errs
}
