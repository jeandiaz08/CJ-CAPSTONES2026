// Package domain - Órdenes, estados y reglas (sin dependencias técnicas).
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	ErrNotFound          = errors.New("orden no encontrada")
	ErrInvalid           = errors.New("orden inválida")
	ErrInvalidTransition = errors.New("transición de estado no permitida")
	ErrConflict          = errors.New("la orden fue modificada por otra operación; recarga e intenta de nuevo")
	ErrDuplicate         = errors.New("ya existe una orden con ese origen y referencia")
	ErrForbidden         = errors.New("tu rol o permisos no permiten esta acción")
	ErrNotEditable       = errors.New("la orden ya fue enviada a preparación: no se puede modificar desde Nexus")
)

// FieldError describe un dato inválido con su motivo (se muestra por campo o por fila).
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors agrupa los problemas de una orden.
type ValidationErrors []FieldError

func (v ValidationErrors) Error() string {
	parts := make([]string, 0, len(v))
	for _, e := range v {
		parts = append(parts, e.Field+": "+e.Message)
	}
	return strings.Join(parts, "; ")
}
func (v ValidationErrors) Unwrap() error { return ErrInvalid }

type Item struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// Tipos de despacho (catálogo dispatch_types del Core).
var DispatchTypes = []string{"domicilio", "retiro_oficina", "express"}

// Destination - destinatario y dirección de entrega.
type Destination struct {
	RecipientName  string `json:"recipientName"`
	RecipientPhone string `json:"recipientPhone"`
	RecipientEmail string `json:"recipientEmail"`
	AddressLine    string `json:"addressLine"`
	Commune        string `json:"commune"`
	RegionCode     string `json:"regionCode"`
}

type Order struct {
	ID          int64
	CompanyID   int64
	ExternalRef string // referencia del canal (marketplace, e-commerce...)
	Channel     string // manual, mercadolibre, shopify...
	Courier     string // código del catálogo couriers
	CourierCode string // código o seguimiento que entrega el courier
	Destination
	DispatchType  string
	Warehouse     string
	CustomerNotes string
	Source        string // manual, bulk, webhook

	Status       Status
	PrevStatus   Status // estado donde ocurrió la incidencia (para volver)
	IncidentKind string
	Priority     bool   // «Enviar primero a la cola» (RF-15)
	WaveID       string // ola de Invas asignada
	WMSRef       string // referencia en Invas (OS)
	Version      int    // control de concurrencia optimista
	Items        []Item
	Note         string // motivo de la incidencia/anulación/cancelación vigente

	ValidationAttempts int
	NextRetryAt        *time.Time
	LastError          string

	ClaimedBy     *int64
	ClaimedByName string
	ClaimedAt     *time.Time
	CreatedBy     *int64
	ImportID      *int64

	CreatedAt    time.Time
	UpdatedAt    time.Time
	HandedOverAt *time.Time
	CancelledAt  *time.Time
}

// Draft son los datos de ingreso (manual, carga masiva o integración).
type Draft struct {
	CompanyID   int64  `json:"companyId"`
	ExternalRef string `json:"externalRef"`
	Channel     string `json:"channel"`
	Courier     string `json:"courier"`
	Destination
	DispatchType  string `json:"dispatchType"`
	Warehouse     string `json:"warehouse"`
	CustomerNotes string `json:"customerNotes"`
	Items         []Item `json:"items"`
}

var (
	reEmail  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	rePhone  = regexp.MustCompile(`^\+?[0-9 ()-]{8,20}$`)
	reNumber = regexp.MustCompile(`\d`)
	reRef    = regexp.MustCompile(`^[A-Za-z0-9._:/#-]{1,80}$`)
)

// Normalize limpia el borrador y valida estructura (no consulta catálogos:
// productos, comuna y cobertura los revisa el validador automático).
func (d *Draft) Normalize() ValidationErrors {
	var errs ValidationErrors
	add := func(f, m string) { errs = append(errs, FieldError{f, m}) }

	d.ExternalRef = strings.TrimSpace(d.ExternalRef)
	d.Channel = strings.ToLower(strings.TrimSpace(d.Channel))
	if d.Channel == "" {
		d.Channel = "manual"
	}
	if d.ExternalRef == "" && d.Channel == "manual" {
		d.ExternalRef = GenerateRef()
	}
	d.Courier = strings.ToLower(strings.TrimSpace(d.Courier))
	d.RecipientName = strings.TrimSpace(d.RecipientName)
	d.RecipientPhone = strings.TrimSpace(d.RecipientPhone)
	d.RecipientEmail = strings.ToLower(strings.TrimSpace(d.RecipientEmail))
	d.AddressLine = strings.Join(strings.Fields(d.AddressLine), " ")
	d.Commune = strings.Join(strings.Fields(d.Commune), " ")
	d.RegionCode = strings.ToUpper(strings.TrimSpace(d.RegionCode))
	d.DispatchType = strings.ToLower(strings.TrimSpace(d.DispatchType))
	if d.DispatchType == "" {
		d.DispatchType = "domicilio"
	}
	d.Warehouse = strings.ToLower(strings.TrimSpace(d.Warehouse))
	if d.Warehouse == "" {
		d.Warehouse = "principal"
	}
	d.CustomerNotes = strings.TrimSpace(d.CustomerNotes)

	if d.CompanyID <= 0 {
		add("companyId", "indica la empresa")
	}
	if !reRef.MatchString(d.ExternalRef) {
		add("externalRef", "referencia obligatoria (máx. 80 caracteres, sin espacios)")
	}
	if d.Courier == "" {
		add("courier", "indica el courier")
	}
	if d.RecipientName == "" || len(d.RecipientName) > 120 {
		add("recipientName", "el destinatario es obligatorio (máx. 120 caracteres)")
	}
	if d.RecipientPhone != "" && !rePhone.MatchString(d.RecipientPhone) {
		add("recipientPhone", "teléfono inválido")
	}
	if d.RecipientEmail != "" && !reEmail.MatchString(d.RecipientEmail) {
		add("recipientEmail", "correo inválido")
	}
	if d.DispatchType == "retiro_oficina" {
		if d.Commune == "" {
			add("commune", "indica la comuna de la oficina de retiro")
		}
	} else {
		if len(d.AddressLine) < 5 || !reNumber.MatchString(d.AddressLine) {
			add("addressLine", "la dirección debe incluir calle y número")
		}
		if d.Commune == "" {
			add("commune", "la comuna es obligatoria")
		}
	}
	if !slices.Contains(DispatchTypes, d.DispatchType) {
		add("dispatchType", "tipo de despacho desconocido")
	}
	if len(d.CustomerNotes) > 500 {
		add("customerNotes", "máximo 500 caracteres")
	}
	if len(d.Items) == 0 {
		add("items", "agrega al menos un producto")
	}
	merged := map[string]int{}
	order := []string{}
	for i, it := range d.Items {
		sku := strings.ToUpper(strings.TrimSpace(it.SKU))
		if sku == "" || it.Quantity <= 0 || it.Quantity > 100000 {
			add(fmt.Sprintf("items[%d]", i), "SKU y cantidad (mayor que 0) son obligatorios")
			continue
		}
		if _, seen := merged[sku]; !seen {
			order = append(order, sku)
		}
		merged[sku] += it.Quantity // el mismo SKU en dos líneas se suma
	}
	d.Items = d.Items[:0]
	for _, sku := range order {
		d.Items = append(d.Items, Item{SKU: sku, Quantity: merged[sku]})
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// NewOrder crea una orden Pendiente a partir de un borrador válido.
func NewOrder(d Draft, source string) (Order, error) {
	if errs := d.Normalize(); errs != nil {
		return Order{}, errs
	}
	return Order{CompanyID: d.CompanyID, ExternalRef: d.ExternalRef, Channel: d.Channel, Courier: d.Courier,
		Destination: d.Destination, DispatchType: d.DispatchType, Warehouse: d.Warehouse,
		CustomerNotes: d.CustomerNotes, Source: source, Status: StatusPending, Version: 1, Items: d.Items}, nil
}

func GenerateRef() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "NX-" + time.Now().Format("060102") + "-" + strings.ToUpper(hex.EncodeToString(b))
}

// ---------------------------------------------------------------------------
// Edición controlada (RF-14, H-09)
// ---------------------------------------------------------------------------

// Editable: solo antes de que la orden llegue a Invas. Una orden Validada con
// ola solicitada ya está en la cola de Invas y no se modifica desde Nexus.
func (o Order) Editable() bool {
	switch o.Status {
	case StatusPending, StatusValidating, StatusVoided:
		return true
	case StatusValidated:
		return o.WaveID == ""
	case StatusIncident:
		switch o.PrevStatus {
		case StatusPending, StatusValidating, StatusValidated, StatusReadyPickup:
			// Lista para retiro: se puede corregir dirección o courier (CU-03),
			// pero no productos (ver ItemsEditable).
			return o.WaveID == "" || o.PrevStatus == StatusReadyPickup
		}
	}
	return false
}

// ItemsEditable: productos y cantidades solo antes de llegar a Invas.
func (o Order) ItemsEditable() bool {
	if !o.Editable() {
		return false
	}
	return !(o.Status == StatusIncident && o.PrevStatus == StatusReadyPickup)
}

// Changes - edición parcial.
type Changes struct {
	Courier       *string      `json:"courier"`
	CourierCode   *string      `json:"courierCode"`
	Destination   *Destination `json:"destination"`
	DispatchType  *string      `json:"dispatchType"`
	CustomerNotes *string      `json:"customerNotes"`
	Items         *[]Item      `json:"items"`
}

// ApplyChanges devuelve la orden editada y la lista legible de cambios.
// Valida con las mismas reglas del ingreso.
func (o Order) ApplyChanges(c Changes) (Order, []string, error) {
	if !o.Editable() {
		return o, nil, ErrNotEditable
	}
	if c.Items != nil && !o.ItemsEditable() {
		return o, nil, ErrNotEditable
	}
	d := Draft{CompanyID: o.CompanyID, ExternalRef: o.ExternalRef, Channel: o.Channel, Courier: o.Courier,
		Destination: o.Destination, DispatchType: o.DispatchType, Warehouse: o.Warehouse,
		CustomerNotes: o.CustomerNotes, Items: append([]Item(nil), o.Items...)}
	if c.Courier != nil {
		d.Courier = *c.Courier
	}
	if c.Destination != nil {
		d.Destination = *c.Destination
	}
	if c.DispatchType != nil {
		d.DispatchType = *c.DispatchType
	}
	if c.CustomerNotes != nil {
		d.CustomerNotes = *c.CustomerNotes
	}
	if c.Items != nil {
		d.Items = append([]Item(nil), (*c.Items)...)
	}
	if errs := d.Normalize(); errs != nil {
		return o, nil, errs
	}
	var diff []string
	ch := func(label, from, to string) {
		if from != to {
			diff = append(diff, fmt.Sprintf("%s: «%s» → «%s»", label, from, to))
		}
	}
	n := o
	ch("Courier", o.Courier, d.Courier)
	ch("Destinatario", o.RecipientName, d.RecipientName)
	ch("Teléfono", o.RecipientPhone, d.RecipientPhone)
	ch("Correo", o.RecipientEmail, d.RecipientEmail)
	ch("Dirección", o.AddressLine, d.AddressLine)
	ch("Comuna", o.Commune, d.Commune)
	ch("Región", o.RegionCode, d.RegionCode)
	ch("Tipo de despacho", o.DispatchType, d.DispatchType)
	ch("Notas", o.CustomerNotes, d.CustomerNotes)
	if c.CourierCode != nil {
		code := strings.TrimSpace(*c.CourierCode)
		ch("Código del courier", o.CourierCode, code)
		n.CourierCode = code
	}
	if fmtItems(o.Items) != fmtItems(d.Items) {
		diff = append(diff, fmt.Sprintf("Productos: %s → %s", fmtItems(o.Items), fmtItems(d.Items)))
	}
	n.Courier, n.Destination, n.DispatchType, n.CustomerNotes, n.Items = d.Courier, d.Destination, d.DispatchType, d.CustomerNotes, d.Items
	return n, diff, nil
}

// NeedsRevalidation: cambios que afectan stock, destino o courier obligan a
// validar de nuevo (y liberar el comprometido si ya estaba Validada).
func (o Order) NeedsRevalidation(after Order) bool {
	return fmtItems(o.Items) != fmtItems(after.Items) || o.Commune != after.Commune ||
		o.AddressLine != after.AddressLine || o.Courier != after.Courier || o.DispatchType != after.DispatchType
}

func fmtItems(items []Item) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprintf("%s×%d", it.SKU, it.Quantity))
	}
	slices.Sort(parts)
	return strings.Join(parts, ", ")
}

// ---------------------------------------------------------------------------
// Máquina de estados
// ---------------------------------------------------------------------------

// Apply aplica un evento usando la máquina de estados (PA-04).
func (o *Order) Apply(ev Event) (from Status, err error) {
	from = o.Status
	if ev == EvResolve {
		if o.Status != StatusIncident || o.PrevStatus == "" {
			return from, ErrInvalidTransition
		}
		o.Status, o.PrevStatus, o.IncidentKind = o.PrevStatus, "", ""
		return from, nil
	}
	next, ok := NextStatus(o.Status, ev)
	if !ok {
		return from, ErrInvalidTransition
	}
	if next == StatusIncident {
		o.PrevStatus = o.Status
	}
	if next != StatusIncident {
		o.IncidentKind = ""
	}
	o.Status = next
	return from, nil
}

// Incident pasa la orden a Incidencia con su tipo.
func (o *Order) Incident(kind string) (Status, error) {
	from, err := o.Apply(EvIncident)
	if err == nil {
		o.IncidentKind = kind
	}
	return from, err
}

// InInvas indica que la orden ya está (o puede estar) en una ola de Invas.
func (o Order) InInvas() bool {
	switch o.Status {
	case StatusPicking, StatusPicked, StatusPacking, StatusReadyPickup:
		return true
	case StatusValidated:
		return o.WaveID != ""
	case StatusIncident:
		return o.WaveID != "" || slices.Contains([]Status{StatusPicking, StatusPicked, StatusPacking, StatusReadyPickup}, o.PrevStatus)
	}
	return false
}
