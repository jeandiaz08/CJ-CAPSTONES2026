package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

const labelWorkerActor = "system:courier"

// LabelStrategy - cómo obtiene la etiqueta cada courier (H-12 T09, PA-03).
type LabelStrategy interface {
	// Printable indica si Nexus obtiene e imprime la etiqueta.
	Printable() bool
	Fetch(ctx context.Context, o domain.Order, packages []domain.Package, attempt int) (pdf []byte, trackingCode string, err error)
}

// apiLabel: el courier entrega la etiqueta por API, quizás después de un tiempo.
type apiLabel struct{ gateway ports.CourierGateway }

func (apiLabel) Printable() bool { return true }

func (l apiLabel) Fetch(ctx context.Context, o domain.Order, packages []domain.Package, attempt int) ([]byte, string, error) {
	return l.gateway.Label(ctx, o, packages, attempt)
}

// officeLabel: «Buscar en oficina». Nexus no llama a nadie ni genera un PDF.
type officeLabel struct{}

func (officeLabel) Printable() bool { return false }

func (officeLabel) Fetch(context.Context, domain.Order, []domain.Package, int) ([]byte, string, error) {
	return nil, "", errors.New("este courier no entrega etiqueta por API")
}

func (s *Service) labelStrategy(o domain.Order) LabelStrategy {
	if o.LabelStrategy == domain.LabelStrategyOffice {
		return officeLabel{}
	}
	return apiLabel{gateway: s.Couriers}
}

// LabelResult: con Document, la etiqueta está lista para imprimir; sin él, sigue pendiente.
type LabelResult struct {
	Order    domain.Order
	Document *ports.Document
}

// PrintLabel entrega la etiqueta si ya está lista, o intenta obtenerla. Si el courier
// todavía no la entrega, queda pendiente y LabelRetry la sigue pidiendo (CU-04, RF-23).
func (s *Service) PrintLabel(ctx context.Context, id tenant.Identity, scope []int64, orderID int64) (LabelResult, error) {
	if !canPack(id) && !id.HasPermission(tenant.PermDespacho) {
		return LabelResult{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return LabelResult{}, err
	}
	if o.Status != domain.StatusPacking && o.Status != domain.StatusReadyPickup {
		return LabelResult{}, fail(domain.ErrInvalidTransition, "La etiqueta se imprime en empaque o con la orden Lista para retiro.")
	}
	if !s.labelStrategy(o).Printable() {
		return LabelResult{}, fail(domain.ErrConflict, "Este courier no entrega etiqueta por API: hay que buscarla en la oficina.")
	}
	pk, err := s.Fulfillment.Packages(ctx, o.ID)
	if err != nil {
		return LabelResult{}, err
	}
	if len(pk) == 0 {
		return LabelResult{}, domain.ValidationErrors{{Field: "packages", Message: "Registra los bultos antes de imprimir la etiqueta"}}
	}
	if o.LabelStatus != domain.LabelPending && o.LabelStatus != domain.LabelReady {
		if err := s.Fulfillment.RequestLabel(ctx, o, id.Actor()); err != nil {
			return LabelResult{}, err
		}
		attempt, claimed, err := s.Fulfillment.ClaimLabel(ctx, o.ID)
		if err != nil {
			return LabelResult{}, err
		}
		if claimed {
			if err := s.attemptLabel(ctx, o, pk, attempt, id.Actor(), false); err != nil {
				return LabelResult{}, err
			}
		}
		if o, err = s.Orders.Get(ctx, scope, orderID); err != nil {
			return LabelResult{}, err
		}
	}
	if o.LabelStatus != domain.LabelReady || o.LabelDocumentID == nil {
		return LabelResult{Order: o}, nil
	}
	doc, err := s.Fulfillment.Document(ctx, o.ID, *o.LabelDocumentID)
	if err != nil {
		return LabelResult{}, err
	}
	_ = s.Orders.AddEvent(ctx, o.ID, "print", "label", id.Actor(), fmt.Sprintf("Etiqueta %s impresa (%d bulto/s)", o.Courier, len(pk)))
	return LabelResult{Order: o, Document: &doc}, nil
}

// attemptLabel hace un intento ya reclamado. background: lo hizo el Worker y nadie espera la
// respuesta, así que el resultado se avisa también como notificación.
func (s *Service) attemptLabel(ctx context.Context, o domain.Order, pk []domain.Package, attempt int, actor string, background bool) error {
	pdf, tracking, err := s.labelStrategy(o).Fetch(ctx, o, pk, attempt)
	var key string
	if err == nil {
		key = fmt.Sprintf("%d/%d/%s.pdf", o.CompanyID, o.ID, NewID())
		_, err = s.Files.Put(ctx, key, bytes.NewReader(pdf))
	}
	if err != nil {
		return s.labelNotReady(ctx, o, attempt, actor, background, err)
	}
	sum := sha256.Sum256(pdf)
	done, err := s.Fulfillment.CompleteLabel(ctx, ports.Document{OrderID: o.ID, CompanyID: o.CompanyID, Kind: "etiqueta",
		Filename: fmt.Sprintf("etiqueta-%s-%d.pdf", o.Courier, o.ID), ContentType: "application/pdf",
		SizeBytes: int64(len(pdf)), SHA256: hex.EncodeToString(sum[:]), StorageKey: key, UploadedBy: actor}, tracking, len(pk), actor)
	if err != nil || !done { // !done: la invalidó un cambio de bultos mientras se pedía
		return err
	}
	s.Notifier.OrderChanged(ctx, o)
	if background {
		s.notices.OnLabel(ctx, o, true)
	}
	return nil
}

func (s *Service) labelNotReady(ctx context.Context, o domain.Order, attempt int, actor string, background bool, cause error) error {
	if !errors.Is(cause, domain.ErrLabelNotReady) {
		s.Log.Warn("no se pudo obtener la etiqueta", "order_id", o.ID, "attempt", attempt, "err", cause)
	}
	if attempt >= domain.LabelMaxAttempts {
		if err := s.Fulfillment.FailLabel(ctx, o.ID, attempt, cause.Error(), actor); err != nil {
			return err
		}
		s.Notifier.OrderChanged(ctx, o)
		s.notices.OnLabel(ctx, o, false)
		return nil
	}
	if err := s.Fulfillment.RetryLabel(ctx, o.ID, attempt, domain.LabelRetryDelay(attempt), cause.Error()); err != nil {
		return err
	}
	if attempt == 1 {
		_ = s.Orders.AddEvent(ctx, o.ID, "label", "pending", actor, "Etiqueta pendiente: se reintenta sola")
		s.Notifier.OrderChanged(ctx, o)
	}
	return nil
}

// requestLabelAfterPacking: una orden confirmada sin etiqueta la pide sola. No bloquea la
// confirmación, y una etiqueta pendiente o fallida se queda como está.
func (s *Service) requestLabelAfterPacking(ctx context.Context, o domain.Order, actor string) {
	if !s.labelStrategy(o).Printable() || o.LabelStatus != domain.LabelNone {
		return
	}
	if err := s.Fulfillment.RequestLabel(ctx, o, actor); err != nil {
		s.Log.Warn("no se pudo pedir la etiqueta", "order_id", o.ID, "err", err)
	}
}

// LabelRetry pide las etiquetas pendientes cuando les toca (H-17 escenario 3): lo pendiente
// está en la base, así que un reinicio retoma el trabajo sin repetir lo que ya llegó.
type LabelRetry struct{ svc *Service }

func NewLabelRetry(svc *Service) *LabelRetry { return &LabelRetry{svc: svc} }

func (l *LabelRetry) RunOnce(ctx context.Context) {
	s := l.svc
	claims, err := s.Fulfillment.ClaimDueLabels(ctx, 20)
	if err != nil {
		s.Log.Warn("no se pudieron leer las etiquetas pendientes", "err", err)
		return
	}
	for _, c := range claims {
		if err := l.process(ctx, c); err != nil {
			s.Log.Warn("reintento de etiqueta", "order_id", c.OrderID, "err", err)
		}
	}
}

func (l *LabelRetry) process(ctx context.Context, c ports.LabelClaim) error {
	s := l.svc
	o, err := s.Orders.Get(ctx, nil, c.OrderID)
	if err != nil {
		return err
	}
	pk, err := s.Fulfillment.Packages(ctx, o.ID)
	if err != nil {
		return err
	}
	// Ya no corresponde (cancelada, en Incidencia): se vuelve a pedir si regresa.
	if (o.Status != domain.StatusPacking && o.Status != domain.StatusReadyPickup) || len(pk) == 0 {
		return s.Fulfillment.ClearLabel(ctx, o.ID)
	}
	return s.attemptLabel(ctx, o, pk, c.Attempt, labelWorkerActor, true)
}
