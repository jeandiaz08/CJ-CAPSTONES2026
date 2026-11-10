package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
	"github.com/nexuswst/backend-core/internal/shared/wave"
)

// Catalog - página Productos (Tarea 09, H-04: SCRUM-101, 102, 105, 106 y 236). Crea, edita, activa
// y desactiva productos y kits; cada cambio queda en el Registro de acciones › Productos. Reutiliza
// el repositorio y las reglas de kits de Service (componentes simples, activos y de la misma
// empresa). Un kit no tiene stock propio: solo se muestra su composición. Quién puede escribir lo
// decide la ruta (solo el Administrador); el alcance por empresa llega en companyIDs.
type Catalog struct {
	products ports.ProductRepository
	repo     ports.CatalogRepository
}

func NewCatalog(products ports.ProductRepository, repo ports.CatalogRepository) *Catalog {
	return &Catalog{products: products, repo: repo}
}

// ProductView es lo que ve la pantalla. Components solo en el detalle de un kit.
type ProductView struct {
	ID          int64           `json:"id"`
	CompanyID   int64           `json:"companyId"`
	CompanyName string          `json:"companyName"`
	SKU         string          `json:"sku"`
	Description string          `json:"description"`
	Type        string          `json:"type"`
	Active      bool            `json:"active"`
	WaveCode    *string         `json:"waveCode"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	UpdatedBy   string          `json:"updatedBy"` // quién hizo la última acción (vacío si no hay)
	Components  []ComponentView `json:"components,omitempty"`
}

type ComponentView struct {
	ProductID   int64  `json:"productId"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
}

func catalogView(it ports.CatalogItem) ProductView {
	return ProductView{ID: it.ID, CompanyID: it.CompanyID, CompanyName: it.CompanyName, SKU: it.SKU, Description: it.Description,
		Type: string(it.Type), Active: it.Active, WaveCode: it.WaveCode, CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt, UpdatedBy: it.UpdatedByName}
}

func (c *Catalog) List(ctx context.Context, f ports.CatalogFilter) ([]ProductView, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 20
	}
	f.Offset = max(f.Offset, 0)
	items, total, err := c.repo.ListCatalog(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ProductView, 0, len(items))
	for _, it := range items {
		out = append(out, catalogView(it))
	}
	return out, total, nil
}

// Get devuelve el producto si está en el alcance; si es kit, con su composición.
func (c *Catalog) Get(ctx context.Context, companyIDs []int64, id int64) (ProductView, error) {
	it, err := c.repo.GetCatalog(ctx, companyIDs, id)
	if err != nil {
		return ProductView{}, err
	}
	v := catalogView(it)
	if it.Type == domain.TypeKit {
		comps, err := c.products.Components(ctx, it.ID)
		if err != nil {
			return ProductView{}, err
		}
		v.Components = make([]ComponentView, 0, len(comps))
		for _, k := range comps {
			v.Components = append(v.Components, ComponentView{ProductID: k.ComponentID, SKU: k.SKU, Description: k.Description, Quantity: k.Quantity})
		}
	}
	return v, nil
}

// waveErrors valida la ola (nil o vacía = «General (según reglas)»).
func waveErrors(code *string) (*string, domain.ValidationErrors) {
	w, err := wave.Normalize(code)
	if err != nil {
		return nil, domain.ValidationErrors{{Field: "waveCode", Message: "Ola inválida: usa el código de la ola de Invas o «General (según reglas)»."}}
	}
	return w, nil
}

// composition valida los componentes de un kit con las reglas de Service: SKU del catálogo de la
// misma empresa, activos, simples y sin repetir; al menos uno.
func (c *Catalog) composition(ctx context.Context, kit domain.Product, in []domain.ComponentInput) ([]domain.KitComponent, error) {
	lines, err := domain.NormalizeComponents(kit.SKU, in)
	if err != nil {
		return nil, componentsField(err)
	}
	skus := make([]string, len(lines))
	for i, l := range lines {
		skus[i] = l.SKU
	}
	found, err := c.products.BySKUs(ctx, kit.CompanyID, skus)
	if err != nil {
		return nil, err
	}
	bySKU := make(map[string]domain.Product, len(found))
	for _, p := range found {
		bySKU[p.SKU] = p
	}
	comps := make([]domain.KitComponent, 0, len(lines))
	for _, l := range lines {
		p, ok := bySKU[l.SKU]
		if !ok {
			return nil, componentsField(&domain.ComponentError{SKU: l.SKU, Reason: "no existe en el catálogo del cliente"})
		}
		if err := domain.ValidateComponent(kit, p); err != nil {
			return nil, componentsField(err)
		}
		comps = append(comps, domain.KitComponent{ComponentID: p.ID, SKU: p.SKU, Description: p.Description, Quantity: l.Quantity})
	}
	return comps, nil
}

func componentsField(err error) domain.ValidationErrors {
	msg := err.Error()
	return domain.ValidationErrors{{Field: "components", Message: strings.ToUpper(msg[:1]) + msg[1:] + "."}}
}

func compositionText(comps []domain.KitComponent) string {
	parts := make([]string, 0, len(comps))
	for _, k := range comps {
		parts = append(parts, fmt.Sprintf("%s ×%d", k.SKU, k.Quantity))
	}
	return strings.Join(parts, ", ")
}

func waveText(w *string) string {
	if w == nil {
		return "General (según reglas)"
	}
	return *w
}

// Create da de alta un producto o un kit (con su composición) en una transacción.
func (c *Catalog) Create(ctx context.Context, actorID int64, companyIDs []int64, d domain.ProductDraft, in []domain.ComponentInput) (ProductView, error) {
	errs := d.Normalize(true)
	w, werrs := waveErrors(d.WaveCode)
	errs = append(errs, werrs...)
	if d.CompanyID > 0 {
		if !canAccess(companyIDs, d.CompanyID) {
			errs = append(errs, domain.FieldError{Field: "companyId", Message: "El cliente no existe."})
		} else if _, active, err := c.repo.Company(ctx, d.CompanyID); errors.Is(err, domain.ErrNotFound) {
			errs = append(errs, domain.FieldError{Field: "companyId", Message: "El cliente no existe."})
		} else if err != nil {
			return ProductView{}, err
		} else if !active {
			errs = append(errs, domain.FieldError{Field: "companyId", Message: "El cliente está desactivado: actívalo antes de agregarle productos."})
		}
	}
	if d.Type == domain.TypeSimple && len(in) > 0 {
		errs = append(errs, domain.FieldError{Field: "components", Message: "Solo un kit tiene composición."})
	}
	if len(errs) > 0 {
		return ProductView{}, errs
	}
	p := domain.Product{CompanyID: d.CompanyID, SKU: d.SKU, Description: d.Description, Type: d.Type, Active: true, WaveCode: w}
	var comps []domain.KitComponent
	if p.Type == domain.TypeKit {
		var err error
		if comps, err = c.composition(ctx, p, in); err != nil {
			return ProductView{}, err
		}
	}
	detail := map[string]any{"sku": p.SKU, "description": p.Description, "productType": string(p.Type), "waveCode": waveText(w)}
	if len(comps) > 0 {
		detail["components"] = compositionText(comps)
	}
	created, err := c.repo.CreateProduct(ctx, p, comps, ports.Audit{ActorID: actorID, Action: "product_created", Detail: detail})
	if errors.Is(err, domain.ErrDuplicateSKU) {
		return ProductView{}, DuplicateSKUError{}
	}
	if err != nil {
		return ProductView{}, err
	}
	return c.Get(ctx, nil, created.ID)
}

// DuplicateSKUError: 409 con el error en el campo.
type DuplicateSKUError struct{}

func (DuplicateSKUError) Error() string { return "Ese SKU ya existe para el cliente." }
func (DuplicateSKUError) Unwrap() error { return domain.ErrDuplicateSKU }

func canAccess(companyIDs []int64, id int64) bool {
	if companyIDs == nil {
		return true
	}
	for _, c := range companyIDs {
		if c == id {
			return true
		}
	}
	return false
}

// Update edita descripción, ola y, si es kit, la composición (in nil = no cambia). Cliente, SKU y
// tipo no cambian. Deja una sola acción con el antes y el después.
func (c *Catalog) Update(ctx context.Context, actorID int64, companyIDs []int64, id int64, d domain.ProductDraft, in []domain.ComponentInput) (ProductView, error) {
	cur, err := c.Get(ctx, companyIDs, id)
	if err != nil {
		return ProductView{}, err
	}
	errs := d.Normalize(false)
	w, werrs := waveErrors(d.WaveCode)
	errs = append(errs, werrs...)
	if cur.Type != string(domain.TypeKit) && in != nil {
		errs = append(errs, domain.FieldError{Field: "components", Message: "Solo un kit tiene composición."})
	}
	if len(errs) > 0 {
		return ProductView{}, errs
	}
	diff := map[string]any{}
	if cur.Description != d.Description {
		diff["description"] = map[string]any{"from": cur.Description, "to": d.Description}
	}
	if waveText(cur.WaveCode) != waveText(w) {
		diff["waveCode"] = map[string]any{"from": waveText(cur.WaveCode), "to": waveText(w)}
	}
	var comps []domain.KitComponent
	if in != nil {
		kit := domain.Product{ID: cur.ID, CompanyID: cur.CompanyID, SKU: cur.SKU, Type: domain.TypeKit}
		if comps, err = c.composition(ctx, kit, in); err != nil {
			return ProductView{}, err
		}
		before := make([]domain.KitComponent, 0, len(cur.Components))
		for _, k := range cur.Components {
			before = append(before, domain.KitComponent{SKU: k.SKU, Quantity: k.Quantity})
		}
		if compositionText(before) != compositionText(comps) {
			diff["components"] = map[string]any{"from": compositionText(before), "to": compositionText(comps)}
		} else {
			comps = nil // sin cambios en la composición
		}
	}
	if len(diff) == 0 {
		return cur, nil
	}
	diff["sku"] = cur.SKU
	if err := c.repo.UpdateProduct(ctx, id, d.Description, w, comps, ports.Audit{ActorID: actorID, Action: "product_updated", Detail: diff}); err != nil {
		return ProductView{}, err
	}
	return c.Get(ctx, nil, id)
}

// SetActive activa o desactiva (desactivar exige motivo). No se desactiva un componente de kits
// activos, ni se activa un kit con componentes inactivos.
func (c *Catalog) SetActive(ctx context.Context, actorID int64, companyIDs []int64, id int64, active bool, reason string) (ProductView, error) {
	cur, err := c.Get(ctx, companyIDs, id)
	if err != nil {
		return ProductView{}, err
	}
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); (!active && n < 5) || n > 500 {
		return ProductView{}, domain.ErrReasonRequired
	}
	if cur.Active == active {
		return ProductView{}, domain.ErrSameState
	}
	if !active && cur.Type == string(domain.TypeSimple) {
		kits, err := c.repo.ActiveKitsUsing(ctx, id)
		if err != nil {
			return ProductView{}, err
		}
		if len(kits) > 0 {
			return ProductView{}, domain.InUseError{Kits: kits}
		}
	}
	if active && cur.Type == string(domain.TypeKit) {
		var inactive []string
		for _, k := range cur.Components {
			p, err := c.products.GetByID(ctx, nil, k.ProductID)
			if err != nil {
				return ProductView{}, err
			}
			if !p.Active {
				inactive = append(inactive, p.SKU)
			}
		}
		if len(inactive) > 0 {
			return ProductView{}, domain.InactiveComponentsError{SKUs: inactive}
		}
	}
	action := "product_enabled"
	if !active {
		action = "product_disabled"
	}
	detail := map[string]any{"sku": cur.SKU}
	if reason != "" {
		detail["reason"] = reason
	}
	if err := c.repo.SetActive(ctx, id, active, ports.Audit{ActorID: actorID, Action: action, Detail: detail}); err != nil {
		return ProductView{}, err
	}
	return c.Get(ctx, nil, id)
}
