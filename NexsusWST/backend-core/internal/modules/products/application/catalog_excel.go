package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
	"github.com/nexuswst/backend-core/internal/platform/xlsx"
)

// Excel de Productos (Tarea 09, Fase 2: SCRUM-103, 104 y 107): exportar el catálogo autorizado,
// plantilla e importación masiva de artículos. Los kits solo se exportan (con su composición) y se
// crean a mano: su composición no cabe en una fila simple.

const (
	MaxImportRows  = 500
	MaxExportRows  = 10000
	importExample  = "EJEMPLO"
	productSheet   = "Productos"
	instructionTab = "Instrucciones"
)

var ErrExportTooBig = errors.New("hay demasiados productos para exportar: acota los filtros")

// ImportError es un ValidationError legible (422) de la importación completa (archivo o columnas).
type ImportError struct{ Msg string }

func (e ImportError) Error() string { return e.Msg }

var chile = func() *time.Location {
	if loc, err := time.LoadLocation("America/Santiago"); err == nil {
		return loc
	}
	return time.FixedZone("CLT", -3*3600)
}()

func typeText(t string) string {
	if t == string(domain.TypeKit) {
		return "Kit"
	}
	return "Artículo"
}

func stateText(active bool) string {
	if active {
		return "Activo"
	}
	return "Inactivo"
}

// Export: los mismos filtros de la tabla (o solo los seleccionados), con la composición de los kits.
func (c *Catalog) Export(ctx context.Context, f ports.CatalogFilter, w io.Writer) error {
	f.Limit, f.Offset = MaxExportRows+1, 0
	items, _, err := c.repo.ListCatalog(ctx, f)
	if err != nil {
		return err
	}
	if len(items) > MaxExportRows {
		return ErrExportTooBig
	}
	rows := make([][]any, 0, len(items))
	for _, it := range items {
		composition := ""
		if it.Type == domain.TypeKit {
			comps, err := c.products.Components(ctx, it.ID)
			if err != nil {
				return err
			}
			composition = compositionText(comps)
		}
		rows = append(rows, []any{it.SKU, it.Description, typeText(string(it.Type)), it.CompanyName, waveText(it.WaveCode), stateText(it.Active), composition,
			it.CreatedAt.In(chile).Format("02-01-2006 15:04"), it.UpdatedAt.In(chile).Format("02-01-2006 15:04"), it.UpdatedByName})
	}
	return xlsx.Write(w, productSheet, []string{"SKU", "Descripción", "Tipo", "Cliente", "Ola", "Estado", "Composición (kits)",
		"Fecha de creación", "Fecha de actualización", "Modificado por"}, rows)
}

var importHeader = []string{"RUT cliente", "SKU", "Descripción", "Ola"}

// Template: hoja «Productos» con el encabezado y una fila de ejemplo (no se importa) y hoja
// «Instrucciones» con las reglas y los clientes activos.
func (c *Catalog) Template(ctx context.Context, w io.Writer) error {
	companies, err := c.repo.ActiveCompanies(ctx)
	if err != nil {
		return err
	}
	rows := [][]any{{"76.123.456-0", importExample + "-AO-1001", "Mochila 18 L verde", ""}}
	info := [][]any{
		{"Una fila por artículo. Los kits se crean a mano en «Nuevo producto» (su composición no cabe en una fila).", ""},
		{"Obligatorios: RUT cliente (de un cliente activo), SKU y Descripción.", ""},
		{"SKU: único por cliente, sin espacios, hasta 60 caracteres; se guarda en mayúsculas.", ""},
		{"Ola: el código de la ola de Invas; vacía = «General (según reglas)».", ""},
		{"Las filas cuyo SKU empieza con «EJEMPLO» no se importan.", ""},
		{fmt.Sprintf("Máximo %d filas por archivo. Las filas válidas se crean; las inválidas se informan con su fila y motivo.", MaxImportRows), ""},
		{"", ""},
		{"Clientes activos", "RUT"},
	}
	for _, co := range companies {
		info = append(info, []any{co[0], co[1]})
	}
	return xlsx.WriteWorkbook(w, []xlsx.Sheet{
		{Name: productSheet, Header: importHeader, Rows: rows, Widths: []int{16, 22, 40, 12}},
		{Name: instructionTab, Header: []string{"Instrucciones", ""}, Rows: info, Widths: []int{100, 16}},
	})
}

type ImportRowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type ImportedProduct struct {
	Row  int    `json:"row"`
	ID   int64  `json:"id"`
	Name string `json:"name"` // SKU · descripción (el modal de resultado lo muestra como en perfiles)
}

type ImportResult struct {
	Rows    int               `json:"rows"`
	Created []ImportedProduct `json:"created"`
	Errors  []ImportRowError  `json:"errors"`
}

// Import crea los artículos válidos e informa los inválidos con su fila y motivo: sin RUT, cliente
// inexistente o desactivado, sin SKU, SKU repetido en el archivo o ya existente en el cliente, sin
// descripción u ola inválida. Cada producto creado queda en el registro (origen «Carga masiva»).
func (c *Catalog) Import(ctx context.Context, actorID int64, data []byte) (ImportResult, error) {
	res := ImportResult{Created: []ImportedProduct{}, Errors: []ImportRowError{}}
	table, err := xlsx.ReadFirstSheet(data)
	if err != nil || len(table) == 0 {
		return res, ImportError{"no se pudo leer el archivo: usa la plantilla .xlsx de productos"}
	}
	col := map[string]int{}
	for i, h := range table[0] {
		col[strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(h), "*")))] = i
	}
	for _, h := range importHeader[:3] {
		if _, ok := col[strings.ToLower(h)]; !ok {
			return res, ImportError{fmt.Sprintf("falta la columna «%s»: usa la plantilla de productos", h)}
		}
	}
	if len(table)-1 > MaxImportRows {
		return res, ImportError{fmt.Sprintf("el archivo tiene %d filas; el máximo es %d", len(table)-1, MaxImportRows)}
	}
	cell := func(r []string, name string) string {
		if i, ok := col[strings.ToLower(name)]; ok && i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}
	type company struct {
		id     int64
		active bool
		err    error
	}
	companies := map[string]company{}
	seen := map[string]int{} // empresa|SKU → fila
	for i, r := range table[1:] {
		row := i + 2 // fila de Excel (la 1 es el encabezado)
		sku := domain.NormalizeSKU(cell(r, "SKU"))
		if strings.Join(r, "") == "" || strings.HasPrefix(sku, importExample) {
			continue
		}
		res.Rows++
		fail := func(field, msg string) {
			res.Errors = append(res.Errors, ImportRowError{Row: row, Field: field, Message: msg})
		}
		rut := cell(r, "RUT cliente")
		if rut == "" {
			fail("companyId", "Falta el RUT del cliente.")
			continue
		}
		co, ok := companies[rut]
		if !ok {
			id, _, active, err := c.repo.CompanyByTaxID(ctx, rut)
			co = company{id, active, err}
			companies[rut] = co
		}
		switch {
		case errors.Is(co.err, domain.ErrNotFound):
			fail("companyId", fmt.Sprintf("No hay un cliente con el RUT %s.", rut))
			continue
		case co.err != nil:
			return res, co.err
		case !co.active:
			fail("companyId", fmt.Sprintf("El cliente con RUT %s está desactivado.", rut))
			continue
		}
		var wave *string
		if w := cell(r, "Ola"); w != "" {
			wave = &w
		}
		d := domain.ProductDraft{CompanyID: co.id, SKU: sku, Description: cell(r, "Descripción"), Type: domain.TypeSimple, WaveCode: wave}
		if errs := d.Normalize(true); len(errs) > 0 {
			fail(errs[0].Field, errs[0].Message)
			continue
		}
		key := fmt.Sprintf("%d|%s", co.id, d.SKU)
		if prev, dup := seen[key]; dup {
			fail("sku", fmt.Sprintf("SKU repetido en el archivo (fila %d).", prev))
			continue
		}
		seen[key] = row
		v, err := c.create(ctx, actorID, nil, d, nil, map[string]any{"source": "Carga masiva"})
		if err != nil {
			var ve domain.ValidationErrors
			switch {
			case errors.Is(err, domain.ErrDuplicateSKU):
				fail("sku", "Ese SKU ya existe para el cliente.")
			case errors.As(err, &ve):
				fail(ve[0].Field, ve[0].Message)
			default:
				return res, err
			}
			continue
		}
		res.Created = append(res.Created, ImportedProduct{Row: row, ID: v.ID, Name: v.SKU + " · " + v.Description})
	}
	return res, nil
}
