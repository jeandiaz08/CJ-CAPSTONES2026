package application

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/platform/xlsx"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// Columnas del formato de importación (plantilla descargable). Una orden con
// varios productos usa varias filas con la misma referencia.
var ImportColumns = []string{"referencia", "courier", "tipo_despacho", "destinatario", "telefono", "correo",
	"direccion", "comuna", "region", "notas", "sku", "cantidad"}

const (
	MaxImportBytes = 5 << 20
	MaxImportRows  = 5000
)

type ImportResult struct {
	ImportID      int64            `json:"importId"`
	TotalRows     int              `json:"totalRows"`
	OrdersCreated int              `json:"ordersCreated"`
	OrderIDs      []int64          `json:"orderIds"`
	FailedRows    int              `json:"failedRows"`
	Errors        []ports.RowError `json:"errors"`
}

// ImportOrders - carga masiva (RF-09, H-05 escenario 2): se crean las órdenes
// válidas y se informan las filas inválidas con su motivo, sin perder la carga.
func (s *Service) ImportOrders(ctx context.Context, id tenant.Identity, companyID int64, filename string, r io.Reader) (ImportResult, error) {
	if !canCreate(id) {
		return ImportResult{}, domain.ErrForbidden
	}
	if companyID == 0 && !id.AllCompanies && len(id.Companies) == 1 {
		companyID = id.Companies[0]
	}
	if companyID == 0 {
		return ImportResult{}, fail(domain.ErrInvalid, "Indica la empresa de la carga.")
	}
	if !id.CanAccess(companyID) {
		return ImportResult{}, domain.ErrForbidden
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxImportBytes+1))
	if err != nil {
		return ImportResult{}, err
	}
	if len(data) > MaxImportBytes {
		return ImportResult{}, fail(domain.ErrInvalid, "El archivo supera 5 MB.")
	}
	rows, err := parseTable(filename, data)
	if err != nil {
		return ImportResult{}, fail(domain.ErrInvalid, "%v", err)
	}
	if len(rows) < 2 {
		return ImportResult{}, fail(domain.ErrInvalid, "El archivo no tiene filas de datos. Usa la plantilla.")
	}
	if len(rows)-1 > MaxImportRows {
		return ImportResult{}, fail(domain.ErrInvalid, "Máximo %d filas por carga.", MaxImportRows)
	}
	col, missing := headerIndex(rows[0])
	if len(missing) > 0 {
		return ImportResult{}, fail(domain.ErrInvalid, "Faltan columnas en el encabezado: %s. Usa la plantilla.", strings.Join(missing, ", "))
	}
	get := func(r []string, name string) string {
		if i := col[name]; i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}

	type group struct {
		draft domain.Draft
		rows  []int
		bad   []ports.RowError
	}
	groups := map[string]*group{}
	var orderKeys []string
	res := ImportResult{Errors: []ports.RowError{}, OrderIDs: []int64{}}
	for i, r := range rows[1:] {
		rowNum := i + 2 // fila en la planilla (1 = encabezado)
		if isBlank(r) {
			continue
		}
		res.TotalRows++
		ref := get(r, "referencia")
		if ref == "" {
			res.Errors = append(res.Errors, ports.RowError{Row: rowNum, Message: "falta la referencia"})
			continue
		}
		g := groups[ref]
		if g == nil {
			g = &group{draft: domain.Draft{CompanyID: companyID, ExternalRef: ref, Channel: "manual",
				Courier: get(r, "courier"), DispatchType: get(r, "tipo_despacho"), CustomerNotes: get(r, "notas"),
				Destination: domain.Destination{RecipientName: get(r, "destinatario"), RecipientPhone: get(r, "telefono"),
					RecipientEmail: get(r, "correo"), AddressLine: get(r, "direccion"), Commune: get(r, "comuna"), RegionCode: get(r, "region")}}}
			groups[ref] = g
			orderKeys = append(orderKeys, ref)
		}
		g.rows = append(g.rows, rowNum)
		qty, err := strconv.Atoi(get(r, "cantidad"))
		if err != nil || qty <= 0 {
			g.bad = append(g.bad, ports.RowError{Row: rowNum, Ref: ref, Message: "cantidad inválida: debe ser un entero mayor que 0"})
			continue
		}
		sku := get(r, "sku")
		if sku == "" {
			g.bad = append(g.bad, ports.RowError{Row: rowNum, Ref: ref, Message: "falta el SKU"})
			continue
		}
		g.draft.Items = append(g.draft.Items, domain.Item{SKU: sku, Quantity: qty})
	}

	uid := id.UserID
	for _, ref := range orderKeys {
		g := groups[ref]
		rowsTxt := joinInts(g.rows)
		if len(g.bad) > 0 { // una orden incompleta no se crea a medias
			res.Errors = append(res.Errors, g.bad...)
			continue
		}
		o, err := domain.NewOrder(g.draft, "bulk")
		if err != nil {
			res.Errors = append(res.Errors, ports.RowError{Row: g.rows[0], Ref: ref, Message: fmt.Sprintf("filas %s: %s", rowsTxt, err.Error())})
			continue
		}
		o, err = s.Orders.Create(ctx, o, id.Actor(), &uid)
		switch {
		case errors.Is(err, domain.ErrDuplicate):
			res.Errors = append(res.Errors, ports.RowError{Row: g.rows[0], Ref: ref, Message: "ya existe una orden con esta referencia"})
		case err != nil:
			return res, err
		default:
			res.OrdersCreated++
			res.OrderIDs = append(res.OrderIDs, o.ID)
			s.Notifier.OrderChanged(ctx, o)
		}
	}
	failed := map[int]bool{}
	for _, e := range res.Errors {
		failed[e.Row] = true
	}
	res.FailedRows = len(failed)
	res.ImportID, err = s.Fulfillment.SaveImport(ctx, companyID, filename, res.TotalRows, res.OrdersCreated, res.Errors, id.Actor())
	return res, err
}

func parseTable(filename string, data []byte) ([][]string, error) {
	name := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(name, ".xlsx") || bytes.HasPrefix(data, []byte("PK\x03\x04")):
		return xlsx.ReadFirstSheet(data)
	case strings.HasSuffix(name, ".csv") || strings.HasSuffix(name, ".txt") || utf8.Valid(data):
		data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // BOM de Excel
		sep := ','
		if first, _, _ := bytes.Cut(data, []byte("\n")); bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
			sep = ';' // Excel en español guarda CSV con punto y coma
		}
		r := csv.NewReader(bytes.NewReader(data))
		r.Comma, r.FieldsPerRecord, r.TrimLeadingSpace = sep, -1, true
		return r.ReadAll()
	}
	return nil, errors.New("formato no soportado: usa .xlsx o .csv")
}

func headerIndex(h []string) (map[string]int, []string) {
	idx := map[string]int{}
	for i, name := range h {
		idx[strings.ReplaceAll(domain.NormalizeName(name), " ", "_")] = i
	}
	aliases := map[string][]string{"telefono": {"fono"}, "correo": {"email"}, "direccion": {"direccion_de_entrega"}, "referencia": {"referencia_externa", "ref"}}
	var missing []string
	for _, c := range ImportColumns {
		if _, ok := idx[c]; ok {
			continue
		}
		found := false
		for _, a := range aliases[c] {
			if i, ok := idx[a]; ok {
				idx[c], found = i, true
				break
			}
		}
		if !found && c != "notas" && c != "telefono" && c != "correo" && c != "region" && c != "tipo_despacho" {
			missing = append(missing, c)
		}
		if !found {
			idx[c] = 1 << 20 // columna opcional ausente
		}
	}
	return idx, missing
}

func isBlank(r []string) bool {
	for _, c := range r {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, x := range v {
		s[i] = strconv.Itoa(x)
	}
	return strings.Join(s, ", ")
}

// ImportTemplate - plantilla descargable con un ejemplo.
func ImportTemplate(w io.Writer) error {
	return xlsx.Write(w, "Órdenes", ImportColumns, [][]any{
		{"PED-1001", "chilexpress", "domicilio", "Ana Pérez", "+56 9 1234 5678", "ana@correo.cl", "Av. Irarrázaval 1234, depto 5", "Ñuñoa", "RM", "Entregar en conserjería", "SKU-001", 2},
		{"PED-1001", "chilexpress", "domicilio", "Ana Pérez", "+56 9 1234 5678", "ana@correo.cl", "Av. Irarrázaval 1234, depto 5", "Ñuñoa", "RM", "", "SKU-002", 1},
	})
}

// ---------------------------------------------------------------------------
// Exportación (RF-10, H-08 escenario 3): el listado filtrado o lo seleccionado
// ---------------------------------------------------------------------------

const MaxExportRows = 10000

var exportHeader = []string{"N°", "Referencia", "Empresa", "Canal", "Courier", "Código courier", "Estado", "Prioridad",
	"Destinatario", "Teléfono", "Dirección", "Comuna", "Productos", "Ola", "Motivo", "Creada", "Actualizada"}

func (s *Service) Export(ctx context.Context, f ports.ListFilter, format string, w io.Writer) (int, error) {
	f.Limit, f.Offset = MaxExportRows, 0
	orders, total, err := s.List(ctx, f)
	if err != nil {
		return 0, err
	}
	if total > MaxExportRows {
		return 0, fail(domain.ErrInvalid, "El filtro tiene %d órdenes: exporta como máximo %d (acota fechas o estado).", total, MaxExportRows)
	}
	names := map[int64]string{}
	rows := make([][]any, 0, len(orders))
	loc, _ := time.LoadLocation("America/Santiago")
	if loc == nil {
		loc = time.Local
	}
	for _, o := range orders {
		if _, ok := names[o.CompanyID]; !ok {
			names[o.CompanyID], _ = s.Catalog.CompanyName(ctx, o.CompanyID)
		}
		items := make([]string, 0, len(o.Items))
		for _, it := range o.Items {
			items = append(items, fmt.Sprintf("%s×%d", it.SKU, it.Quantity))
		}
		prio := ""
		if o.Priority {
			prio = "Sí"
		}
		rows = append(rows, []any{o.ID, o.ExternalRef, names[o.CompanyID], o.Channel, o.Courier, o.CourierCode,
			domain.Labels[o.Status], prio, o.RecipientName, o.RecipientPhone, o.AddressLine, o.Commune,
			strings.Join(items, ", "), o.WaveID, o.Note,
			o.CreatedAt.In(loc).Format("2006-01-02 15:04"), o.UpdatedAt.In(loc).Format("2006-01-02 15:04")})
	}
	if format == "csv" {
		_, _ = w.Write([]byte("\xef\xbb\xbf")) // BOM: Excel abre bien los acentos
		cw := csv.NewWriter(w)
		cw.Comma = ';'
		_ = cw.Write(exportHeader)
		for _, r := range rows {
			rec := make([]string, len(r))
			for i, v := range r {
				rec[i] = fmt.Sprint(v)
			}
			_ = cw.Write(rec)
		}
		cw.Flush()
		return len(rows), cw.Error()
	}
	return len(rows), xlsx.Write(w, "Órdenes", exportHeader, rows)
}
