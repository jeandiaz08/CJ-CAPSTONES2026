package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

func canPack(id tenant.Identity) bool {
	return id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleEmpaque)
}

// ---------------------------------------------------------------------------
// Preparación: Invas confirma el avance (H-12 T02)
// ---------------------------------------------------------------------------

// PickingSync consulta a Invas las órdenes «En preparación» y registra las
// que ya están Preparadas (actor Invas). No infiere picking por otras señales.
type PickingSync struct{ svc *Service }

func NewPickingSync(svc *Service) *PickingSync { return &PickingSync{svc: svc} }

func (p *PickingSync) RunOnce(ctx context.Context) {
	s := p.svc
	orders, err := s.Orders.OrdersInStatus(ctx, domain.StatusPicking, 50)
	if err != nil {
		return
	}
	for _, o := range orders {
		done, err := s.WMS.PickingDone(ctx, o)
		if err != nil || !done {
			continue
		}
		from, err := o.Apply(domain.EvPickConfirmed)
		if err != nil {
			continue
		}
		if _, err := s.move(ctx, o, from, domain.EvPickConfirmed, "system:invas", "Invas confirmó la preparación", ports.TransitionOpts{}); err != nil {
			s.Log.Warn("no se pudo registrar la preparación", "order_id", o.ID, "err", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Empaque (CU-04)
// ---------------------------------------------------------------------------

// PackingQueue - órdenes Preparadas y las que el usuario tiene «En empaque».
// Las tomadas por usuarios deshabilitados vuelven a la bandeja.
func (s *Service) PackingQueue(ctx context.Context, id tenant.Identity, scope []int64) ([]domain.Order, error) {
	if !canPack(id) {
		return nil, domain.ErrForbidden
	}
	return s.Fulfillment.PackingQueue(ctx, scope, id.UserID)
}

// TakeForPacking - Empaque toma la orden (pasa a «En empaque»). Si dos
// personas la toman a la vez, la segunda recibe un conflicto.
func (s *Service) TakeForPacking(ctx context.Context, id tenant.Identity, scope []int64, orderID int64) (domain.Order, error) {
	if !canPack(id) {
		return domain.Order{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	uid, now := id.UserID, time.Now()
	switch {
	case o.Status == domain.StatusPacking && o.ClaimedBy != nil && *o.ClaimedBy == uid:
		return o, nil
	case o.Status == domain.StatusPacking && o.ClaimedByName == "":
		// La tomó un usuario hoy deshabilitado: se reasigna (H-02 escenario 2).
		o.ClaimedBy, o.ClaimedAt = &uid, &now
		return s.move(ctx, o, domain.StatusPacking, "take_over", id.Actor(), "Toma la orden de un usuario deshabilitado", ports.TransitionOpts{})
	case o.Status == domain.StatusPacking:
		return domain.Order{}, fail(domain.ErrConflict, "La orden ya la tomó %s.", o.ClaimedByName)
	}
	from, err := o.Apply(domain.EvStartPacking)
	if err != nil {
		return domain.Order{}, fail(domain.ErrInvalidTransition, "Solo se pueden tomar órdenes Preparadas (esta está «%s»).", domain.Labels[o.Status])
	}
	o.ClaimedBy, o.ClaimedAt = &uid, &now
	out, err := s.move(ctx, o, from, domain.EvStartPacking, id.Actor(), "", ports.TransitionOpts{})
	if errors.Is(err, domain.ErrConflict) {
		return domain.Order{}, fail(domain.ErrConflict, "Otra persona tomó la orden recién.")
	}
	return out, err
}

func (s *Service) ReleasePacking(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, reason string) (domain.Order, error) {
	if !canPack(id) {
		return domain.Order{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if o.ClaimedBy != nil && *o.ClaimedBy != id.UserID && !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return domain.Order{}, fail(domain.ErrForbidden, "Solo quien tomó la orden (o el Jefe) puede devolverla.")
	}
	from, err := o.Apply(domain.EvReleasePacking)
	if err != nil {
		return domain.Order{}, err
	}
	o.ClaimedBy, o.ClaimedAt = nil, nil
	return s.move(ctx, o, from, domain.EvReleasePacking, id.Actor(), reason, ports.TransitionOpts{})
}

func (s *Service) mustHoldPacking(id tenant.Identity, o domain.Order) error {
	if o.Status != domain.StatusPacking {
		if o.Status == domain.StatusReadyPickup {
			return fail(domain.ErrConflict, "La orden ya fue confirmada como Lista para retiro.")
		}
		return fail(domain.ErrInvalidTransition, "La orden no está «En empaque».")
	}
	if o.ClaimedBy == nil || *o.ClaimedBy != id.UserID {
		if !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
			return fail(domain.ErrConflict, "La orden la tiene tomada otra persona.")
		}
	}
	return nil
}

// SetPackages registra (reemplaza) los bultos de la orden en empaque.
func (s *Service) SetPackages(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, p []domain.Package) ([]domain.Package, error) {
	if !canPack(id) {
		return nil, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return nil, err
	}
	if err := s.mustHoldPacking(id, o); err != nil {
		return nil, err
	}
	if errs := domain.ValidatePackages(p); errs != nil {
		return nil, errs
	}
	return s.Fulfillment.ReplacePackages(ctx, o.ID, p, id.Actor(), id.UserID)
}

type PackResult struct {
	Order         domain.Order `json:"order"`
	Discrepancies []string     `json:"discrepancies"`
}

// ConfirmPacking - Empaque verificó productos y cantidades. Si no coincide, la
// orden pasa a Incidencia (no se despacha parcialmente). Si el courier no
// admite esa cantidad de bultos, se bloquea y pasa a Incidencia (CU-04).
func (s *Service) ConfirmPacking(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, counted []domain.Item, version int) (PackResult, error) {
	if !canPack(id) {
		return PackResult{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return PackResult{}, err
	}
	if version != 0 && version != o.Version {
		return PackResult{}, fail(domain.ErrConflict, "La orden cambió mientras la verificabas (quizá otra persona ya la confirmó). Recarga.")
	}
	if err := s.mustHoldPacking(id, o); err != nil {
		return PackResult{}, err
	}
	if d := domain.PackingDiscrepancies(o.Items, counted); len(d) > 0 {
		from, _ := o.Incident(domain.IncidentPacking)
		o.ClaimedBy, o.ClaimedAt = nil, nil
		out, err := s.move(ctx, o, from, domain.EvIncident, id.Actor(), "Diferencia en empaque: "+strings.Join(d, "; "), ports.TransitionOpts{})
		return PackResult{Order: out, Discrepancies: d}, err
	}
	pk, err := s.Fulfillment.Packages(ctx, o.ID)
	if err != nil {
		return PackResult{}, err
	}
	if len(pk) == 0 {
		return PackResult{}, domain.ValidationErrors{{Field: "packages", Message: "Registra los bultos antes de confirmar"}}
	}
	cs, err := s.Rules.CourierSettings(ctx, o.Courier)
	if err != nil {
		return PackResult{}, err
	}
	if cs.MaxPackages != nil && len(pk) > *cs.MaxPackages {
		from, _ := o.Incident(domain.IncidentCourierPackage)
		o.ClaimedBy, o.ClaimedAt = nil, nil
		out, err := s.move(ctx, o, from, domain.EvIncident, id.Actor(),
			fmt.Sprintf("El courier %s admite %d bulto(s) y la orden tiene %d. Cambia el courier o divide el envío.", o.Courier, *cs.MaxPackages, len(pk)),
			ports.TransitionOpts{})
		return PackResult{Order: out}, err
	}
	from, err := o.Apply(domain.EvPacked)
	if err != nil {
		return PackResult{}, err
	}
	o.ClaimedBy, o.ClaimedAt = nil, nil
	out, err := s.move(ctx, o, from, domain.EvPacked, id.Actor(), fmt.Sprintf("%d bulto(s) verificados", len(pk)), ports.TransitionOpts{})
	if errors.Is(err, domain.ErrConflict) {
		return PackResult{}, fail(domain.ErrConflict, "La orden ya fue confirmada por otra persona.")
	}
	return PackResult{Order: out, Discrepancies: []string{}}, err
}

// ---------------------------------------------------------------------------
// Documentos (RF-09, H-05 escenario 3)
// ---------------------------------------------------------------------------

const MaxDocumentBytes = 10 << 20 // 10 MB

var allowedDocs = map[string]string{ // tipo detectado -> extensión permitida
	"application/pdf": ".pdf", "image/png": ".png", "image/jpeg": ".jpg", "text/xml": ".xml",
}

var docKinds = []string{"factura", "boleta", "guia_despacho", "etiqueta", "otro"}

// AddDocument valida tipo REAL (por contenido, no por extensión) y tamaño.
func (s *Service) AddDocument(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, kind, filename string, r io.Reader) (ports.Document, error) {
	if !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleCliente, tenant.RoleEmpaque) {
		return ports.Document{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return ports.Document{}, err
	}
	if o.Status == domain.StatusCancelled {
		return ports.Document{}, fail(domain.ErrInvalidTransition, "La orden está Cancelada.")
	}
	if kind == "" {
		kind = "otro"
	}
	ok := false
	for _, k := range docKinds {
		ok = ok || k == kind
	}
	if !ok {
		return ports.Document{}, fail(domain.ErrInvalid, "Tipo de documento desconocido.")
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxDocumentBytes+1))
	if err != nil {
		return ports.Document{}, err
	}
	if len(data) == 0 {
		return ports.Document{}, fail(domain.ErrInvalid, "El archivo está vacío.")
	}
	if len(data) > MaxDocumentBytes {
		return ports.Document{}, fail(domain.ErrInvalid, "El archivo supera el máximo de 10 MB.")
	}
	ctype := detectDocType(data, filename)
	ext, allowed := allowedDocs[ctype]
	if !allowed {
		return ports.Document{}, fail(domain.ErrInvalid, "Tipo de archivo no permitido. Se aceptan PDF, PNG, JPG y XML.")
	}
	sum := sha256.Sum256(data)
	key := fmt.Sprintf("%d/%d/%s%s", o.CompanyID, o.ID, NewID(), ext)
	if _, err := s.Files.Put(ctx, key, bytes.NewReader(data)); err != nil {
		return ports.Document{}, err
	}
	name := sanitizeFilename(filename, ext)
	doc, err := s.Fulfillment.AddDocument(ctx, ports.Document{OrderID: o.ID, CompanyID: o.CompanyID, Kind: kind,
		Filename: name, ContentType: ctype, SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:]),
		StorageKey: key, UploadedBy: id.Actor()})
	if err == nil {
		_ = s.Orders.AddEvent(ctx, o.ID, "document", "attach", id.Actor(), fmt.Sprintf("%s: %s", kind, name))
	}
	return doc, err
}

func detectDocType(data []byte, filename string) string {
	ct := http.DetectContentType(data)
	if i := strings.Index(ct, ";"); i > 0 {
		ct = ct[:i]
	}
	if ct == "text/xml" || (ct == "text/plain" && strings.EqualFold(path.Ext(filename), ".xml") &&
		bytes.HasPrefix(bytes.TrimSpace(data), []byte("<"))) {
		return "text/xml"
	}
	return ct
}

func sanitizeFilename(name, ext string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	var b strings.Builder
	for _, r := range name {
		if r == '/' || r < 32 || r == '"' {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" || out == "." {
		out = "documento" + ext
	}
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}

// OpenDocument entrega el archivo SOLO si la orden pertenece al alcance (RF-03).
func (s *Service) OpenDocument(ctx context.Context, scope []int64, orderID, docID int64) (ports.Document, io.ReadCloser, error) {
	if _, err := s.Orders.Get(ctx, scope, orderID); err != nil {
		return ports.Document{}, nil, err
	}
	d, err := s.Fulfillment.Document(ctx, orderID, docID)
	if err != nil {
		return d, nil, err
	}
	rc, err := s.Files.Open(ctx, d.StorageKey)
	return d, rc, err
}

// PrintLabel obtiene la etiqueta del courier, la guarda como documento y
// registra la impresión en el historial (CU-04, RF-23).
func (s *Service) PrintLabel(ctx context.Context, id tenant.Identity, scope []int64, orderID int64) (ports.Document, error) {
	if !canPack(id) && !id.HasPermission(tenant.PermDespacho) {
		return ports.Document{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return ports.Document{}, err
	}
	if o.Status != domain.StatusPacking && o.Status != domain.StatusReadyPickup {
		return ports.Document{}, fail(domain.ErrInvalidTransition, "La etiqueta se imprime en empaque o con la orden Lista para retiro.")
	}
	pk, err := s.Fulfillment.Packages(ctx, o.ID)
	if err != nil {
		return ports.Document{}, err
	}
	if len(pk) == 0 {
		return ports.Document{}, domain.ValidationErrors{{Field: "packages", Message: "Registra los bultos antes de imprimir la etiqueta"}}
	}
	pdf, tracking, err := s.Couriers.Label(ctx, o, pk)
	if err != nil {
		return ports.Document{}, fail(err, "El courier no entregó la etiqueta: %v", err)
	}
	if o.CourierCode == "" && tracking != "" {
		after := o
		after.CourierCode = tracking
		if saved, err := s.Orders.SaveEdit(ctx, o, after, []string{"Código del courier: " + tracking}, id.Actor(), "", ports.TransitionOpts{}); err == nil {
			o = saved
		}
	}
	sum := sha256.Sum256(pdf)
	key := fmt.Sprintf("%d/%d/%s.pdf", o.CompanyID, o.ID, NewID())
	if _, err := s.Files.Put(ctx, key, bytes.NewReader(pdf)); err != nil {
		return ports.Document{}, err
	}
	doc, err := s.Fulfillment.AddDocument(ctx, ports.Document{OrderID: o.ID, CompanyID: o.CompanyID, Kind: "etiqueta",
		Filename: fmt.Sprintf("etiqueta-%s-%d.pdf", o.Courier, o.ID), ContentType: "application/pdf",
		SizeBytes: int64(len(pdf)), SHA256: hex.EncodeToString(sum[:]), StorageKey: key, UploadedBy: id.Actor()})
	if err == nil {
		_ = s.Orders.AddEvent(ctx, o.ID, "print", "label", id.Actor(), fmt.Sprintf("Etiqueta %s impresa (%d bulto/s)", o.Courier, len(pk)))
	}
	return doc, err
}

// ---------------------------------------------------------------------------
// Retiro y entrega al courier (CU-05): requiere el PERMISO Despacho
// ---------------------------------------------------------------------------

func canDispatch(id tenant.Identity) bool { return id.HasPermission(tenant.PermDespacho) }

func (s *Service) ReadyByCourier(ctx context.Context, id tenant.Identity, scope []int64) ([]ports.CourierQueue, error) {
	if !canDispatch(id) && !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return nil, domain.ErrForbidden
	}
	return s.Fulfillment.ReadyByCourier(ctx, scope)
}

type ManifestResult struct {
	Manifest ports.Manifest `json:"manifest"`
	Excluded []string       `json:"excluded"` // órdenes que no entraron y por qué
}

// CreateManifest arma el manifiesto con las órdenes Listas para retiro del
// courier. Las que el courier no cubre pasan a Incidencia y no entran.
func (s *Service) CreateManifest(ctx context.Context, id tenant.Identity, scope []int64, courier string, orderIDs []int64) (ManifestResult, error) {
	if !canDispatch(id) {
		return ManifestResult{}, fail(domain.ErrForbidden, "Necesitas el permiso Despacho.")
	}
	courier = strings.ToLower(strings.TrimSpace(courier))
	if courier == "" {
		return ManifestResult{}, fail(domain.ErrInvalid, "Indica el courier.")
	}
	orders, _, err := s.Orders.List(ctx, ports.ListFilter{CompanyIDs: scope, Statuses: []string{string(domain.StatusReadyPickup)},
		Courier: courier, IDs: orderIDs, Limit: 500})
	if err != nil {
		return ManifestResult{}, err
	}
	cs, err := s.Rules.CourierSettings(ctx, courier)
	if err != nil {
		return ManifestResult{}, err
	}
	var include []int64
	res := ManifestResult{Excluded: []string{}}
	for _, o := range orders {
		if !cs.CoversAll && !containsFold(cs.Communes, o.Commune) {
			reason := fmt.Sprintf("El courier %s no tiene cobertura en %s", courier, o.Commune)
			if from, err := o.Incident(domain.IncidentCourierCover); err == nil {
				_, _ = s.move(ctx, o, from, domain.EvIncident, id.Actor(), reason+". Corrige dirección o courier.", ports.TransitionOpts{})
			}
			res.Excluded = append(res.Excluded, fmt.Sprintf("#%d: %s (pasó a Incidencia)", o.ID, reason))
			continue
		}
		include = append(include, o.ID)
	}
	if len(include) == 0 {
		return res, fail(domain.ErrInvalid, "No hay órdenes Listas para retiro de %s disponibles para un manifiesto.", courier)
	}
	res.Manifest, err = s.Fulfillment.CreateManifest(ctx, courier, include, id.Actor())
	return res, err
}

func (s *Service) Manifest(ctx context.Context, id tenant.Identity, manifestID int64) (ports.Manifest, error) {
	if !canDispatch(id) && !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return ports.Manifest{}, domain.ErrForbidden
	}
	return s.Fulfillment.Manifest(ctx, manifestID)
}

func (s *Service) Manifests(ctx context.Context, id tenant.Identity, status string) ([]ports.Manifest, error) {
	if !canDispatch(id) && !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return nil, domain.ErrForbidden
	}
	return s.Fulfillment.Manifests(ctx, status, 50)
}

// ScanPackage marca un bulto como verificado (marcándolo o con lector).
func (s *Service) ScanPackage(ctx context.Context, id tenant.Identity, manifestID int64, code string) (ports.ManifestLine, error) {
	if !canDispatch(id) {
		return ports.ManifestLine{}, fail(domain.ErrForbidden, "Necesitas el permiso Despacho.")
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return ports.ManifestLine{}, fail(domain.ErrInvalid, "Indica el código del bulto.")
	}
	return s.Fulfillment.ScanPackage(ctx, manifestID, code, id.Actor())
}

type CloseResult struct {
	Delivered []int64 `json:"delivered"`
	Pending   []int64 `json:"pending"` // bultos incompletos: quedan para el siguiente retiro
}

// CloseManifest - el courier retira: se registran los datos de quien retira y
// las órdenes completas pasan a «Entregada al courier». No se confirma una
// orden con bultos faltantes (H-13 escenario 2).
func (s *Service) CloseManifest(ctx context.Context, id tenant.Identity, manifestID int64, pickerName, pickerID, plate, notes string) (CloseResult, error) {
	if !canDispatch(id) {
		return CloseResult{}, fail(domain.ErrForbidden, "Necesitas el permiso Despacho.")
	}
	pickerName, pickerID, plate = strings.TrimSpace(pickerName), strings.TrimSpace(pickerID), strings.ToUpper(strings.TrimSpace(plate))
	if pickerName == "" {
		return CloseResult{}, fail(domain.ErrInvalid, "Registra el nombre de quien retira.")
	}
	m, err := s.Fulfillment.Manifest(ctx, manifestID)
	if err != nil {
		return CloseResult{}, err
	}
	if m.Status != "open" {
		return CloseResult{}, fail(domain.ErrConflict, "El manifiesto ya está cerrado.")
	}
	res := CloseResult{Delivered: []int64{}, Pending: []int64{}}
	evidence := fmt.Sprintf("Manifiesto #%d · retira %s", m.ID, pickerName)
	if pickerID != "" {
		evidence += " (" + pickerID + ")"
	}
	if plate != "" {
		evidence += " · patente " + plate
	}
	for _, l := range m.Lines {
		if l.Status != "included" {
			continue
		}
		complete := len(l.Packages) > 0
		for _, p := range l.Packages {
			complete = complete && p.Scanned
		}
		o, err := s.Orders.Get(ctx, nil, l.OrderID)
		if err != nil {
			return res, err
		}
		if !complete || o.Status != domain.StatusReadyPickup {
			_ = s.Fulfillment.SetLineStatus(ctx, m.ID, l.OrderID, "pending_packages", "")
			res.Pending = append(res.Pending, l.OrderID)
			continue
		}
		from, err := o.Apply(domain.EvHandOver)
		if err != nil {
			continue
		}
		if _, err := s.move(ctx, o, from, domain.EvHandOver, id.Actor(), fmt.Sprintf("%s · %d bulto(s)", evidence, len(l.Packages)), ports.TransitionOpts{}); err != nil {
			_ = s.Fulfillment.SetLineStatus(ctx, m.ID, l.OrderID, "pending_packages", "")
			res.Pending = append(res.Pending, l.OrderID)
			continue
		}
		_ = s.Fulfillment.SetLineStatus(ctx, m.ID, l.OrderID, "delivered", "")
		res.Delivered = append(res.Delivered, l.OrderID)
	}
	m.PickerName, m.PickerID, m.VehiclePlate, m.Notes = pickerName, pickerID, plate, strings.TrimSpace(notes)
	return res, s.Fulfillment.CloseManifest(ctx, m, id.Actor())
}
