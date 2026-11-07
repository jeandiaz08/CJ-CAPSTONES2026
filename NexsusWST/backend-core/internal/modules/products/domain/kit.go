package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Composición de kits (RF-06, H-04-T01). Un kit no tiene stock propio: su
// disponibilidad sale de sus componentes, que son productos simples de la
// misma empresa. No se permiten kits dentro de kits (así no hay ciclos).

const MaxKitComponents = 50

var (
	ErrNotKit           = errors.New("el producto no es un kit")
	ErrInvalidComponent = errors.New("composición del kit inválida")
)

// KitComponent es una línea de la composición: qué producto y cuántas unidades
// de él lleva una unidad del kit.
type KitComponent struct {
	ComponentID int64
	SKU         string
	Description string
	Quantity    int
}

// ComponentInput es lo que envía el usuario: SKU y cantidad.
type ComponentInput struct {
	SKU      string
	Quantity int
}

// ComponentError explica qué línea falló y por qué; errors.Is(err,
// ErrInvalidComponent) es verdadero.
type ComponentError struct {
	SKU    string
	Reason string
}

func (e *ComponentError) Error() string {
	if e.SKU == "" {
		return e.Reason
	}
	return fmt.Sprintf("%s: %s", e.SKU, e.Reason)
}

func (e *ComponentError) Is(target error) bool { return target == ErrInvalidComponent }

// NormalizeComponents limpia los SKU y valida cantidades, repetidos y tamaño.
// Las reglas que necesitan el catálogo (existe, activo, simple, misma empresa)
// las aplica la capa de aplicación con ValidateComponent.
func NormalizeComponents(kitSKU string, in []ComponentInput) ([]ComponentInput, error) {
	if len(in) == 0 {
		return nil, &ComponentError{Reason: "el kit debe tener al menos un componente"}
	}
	if len(in) > MaxKitComponents {
		return nil, &ComponentError{Reason: fmt.Sprintf("el kit admite como máximo %d componentes", MaxKitComponents)}
	}
	seen := make(map[string]bool, len(in))
	out := make([]ComponentInput, 0, len(in))
	for _, c := range in {
		sku := strings.TrimSpace(strings.ToUpper(c.SKU))
		switch {
		case sku == "":
			return nil, &ComponentError{Reason: "hay un componente sin SKU"}
		case sku == kitSKU:
			return nil, &ComponentError{SKU: sku, Reason: "un kit no puede contenerse a sí mismo"}
		case c.Quantity <= 0:
			return nil, &ComponentError{SKU: sku, Reason: "la cantidad debe ser mayor que 0"}
		case seen[sku]:
			return nil, &ComponentError{SKU: sku, Reason: "componente repetido; suma las cantidades en una sola línea"}
		}
		seen[sku] = true
		out = append(out, ComponentInput{SKU: sku, Quantity: c.Quantity})
	}
	return out, nil
}

// ValidateComponent revisa un producto del catálogo como componente del kit.
func ValidateComponent(kit Product, p Product) error {
	switch {
	case p.CompanyID != kit.CompanyID:
		return &ComponentError{SKU: p.SKU, Reason: "pertenece a otra empresa"}
	case !p.Active:
		return &ComponentError{SKU: p.SKU, Reason: "el producto está inactivo"}
	case p.Type == TypeKit:
		return &ComponentError{SKU: p.SKU, Reason: "no se permiten kits dentro de kits"}
	}
	return nil
}
