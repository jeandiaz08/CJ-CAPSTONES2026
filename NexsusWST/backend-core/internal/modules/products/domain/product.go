// Package domain contiene entidades y reglas de negocio de Productos.
// NO importa HTTP, PostgreSQL, Redis ni Docker (DAS §6.2).
package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("producto no encontrado")
	ErrDuplicateSKU = errors.New("ya existe un producto con ese SKU para la empresa")
	ErrInvalid      = errors.New("producto inválido")
)

type ProductType string

const (
	TypeSimple ProductType = "simple"
	TypeKit    ProductType = "kit" // RF-06: composición de kits
)

type Product struct {
	ID          int64
	CompanyID   int64
	SKU         string
	Description string
	Type        ProductType
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewProduct aplica las reglas de creación.
func NewProduct(companyID int64, sku, description string, t ProductType) (Product, error) {
	sku = strings.TrimSpace(strings.ToUpper(sku))
	description = strings.TrimSpace(description)
	if companyID <= 0 || sku == "" || description == "" {
		return Product{}, ErrInvalid
	}
	if t == "" {
		t = TypeSimple
	}
	if t != TypeSimple && t != TypeKit {
		return Product{}, ErrInvalid
	}
	return Product{CompanyID: companyID, SKU: sku, Description: description, Type: t, Active: true}, nil
}
