package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// «Despachar igual» (H-13-T08, SCRUM-243, PV-09): al cerrar el manifiesto, una
// orden con bultos sin escanear queda para el siguiente retiro, salvo que quien
// tenga el permiso Despacho (no el perfil) autorice despacharla igual con un
// motivo. La autorización queda en el historial de la orden (quién, cuándo,
// qué bultos faltaron, manifiesto y ola) y avisa al personal interno.
//
// Límite conocido: los bultos no registran qué productos lleva cada uno, así
// que se informa qué bultos faltaron, no qué productos iban en ellos.

const minDispatchReason = 5

// DispatchAnyway - autorización para una orden del manifiesto.
type DispatchAnyway struct {
	OrderID int64  `json:"orderId"`
	Reason  string `json:"reason"`
}

// IncompleteDispatch es lo que se guarda en order_events.details.
type IncompleteDispatch struct {
	Kind            string   `json:"kind"` // despacho_incompleto
	ManifestID      int64    `json:"manifestId"`
	Courier         string   `json:"courier"`
	WaveID          string   `json:"waveId,omitempty"`
	Reason          string   `json:"reason"`
	AuthorizedBy    string   `json:"authorizedBy"`
	PackagesTotal   int      `json:"packagesTotal"`
	ScannedPackages []string `json:"scannedPackages"`
	MissingPackages []string `json:"missingPackages"`
}

// canAuthorizeIncomplete: el permiso Despacho, igual que para cerrar el manifiesto
// (cambio de Jean, ver backend_pendientes «Cambios de backend hechos por Jean»).
func canAuthorizeIncomplete(id tenant.Identity) bool {
	return canDispatch(id)
}

// splitPackages separa los códigos escaneados de los faltantes (orden estable).
func splitPackages(pkgs []ports.ManifestBundle) (scanned, missing []string) {
	scanned, missing = []string{}, []string{}
	for _, p := range pkgs {
		if p.Scanned {
			scanned = append(scanned, p.Code)
		} else {
			missing = append(missing, p.Code)
		}
	}
	return scanned, missing
}

// checkDispatchAnyway valida las autorizaciones antes de tocar nada: deben
// venir de quien tenga el permiso Despacho, con motivo, sin repetir órdenes y
// solo para órdenes del manifiesto que de verdad tienen bultos faltantes (y al
// menos uno escaneado).
func checkDispatchAnyway(id tenant.Identity, m ports.Manifest, in []DispatchAnyway) (map[int64]string, error) {
	out := make(map[int64]string, len(in))
	if len(in) == 0 {
		return out, nil
	}
	if !canAuthorizeIncomplete(id) {
		return nil, fail(domain.ErrForbidden, "Necesitas el permiso Despacho para autorizar un despacho con bultos faltantes.")
	}
	lines := make(map[int64]ports.ManifestLine, len(m.Lines))
	for _, l := range m.Lines {
		lines[l.OrderID] = l
	}
	for _, d := range in {
		reason := strings.TrimSpace(d.Reason)
		l, ok := lines[d.OrderID]
		switch {
		case !ok || l.Status != "included":
			return nil, fail(domain.ErrInvalid, "La orden #%d no está para despacho en el manifiesto #%d.", d.OrderID, m.ID)
		case out[d.OrderID] != "":
			return nil, fail(domain.ErrInvalid, "La orden #%d viene repetida.", d.OrderID)
		case len([]rune(reason)) < minDispatchReason:
			return nil, fail(domain.ErrInvalid, "Escribe el motivo para despachar la orden #%d con bultos faltantes.", d.OrderID)
		}
		scanned, missing := splitPackages(l.Packages)
		switch {
		case len(missing) == 0:
			return nil, fail(domain.ErrInvalid, "La orden #%d tiene todos sus bultos escaneados: no necesita autorización.", d.OrderID)
		case len(scanned) == 0:
			return nil, fail(domain.ErrInvalid, "La orden #%d no tiene ningún bulto escaneado: no hay nada que despachar.", d.OrderID)
		}
		out[d.OrderID] = reason
	}
	return out, nil
}

// summarize arma los totales del manifiesto (H-13-T09): órdenes y bultos que
// salen, escaneados y desglose por empresa. Las líneas movidas o canceladas no cuentan.
func (s *Service) summarize(ctx context.Context, m *ports.Manifest) {
	sum := &ports.ManifestSummary{Companies: []ports.ManifestCompany{}}
	byCo := map[int64]*ports.ManifestCompany{}
	for _, l := range m.Lines {
		if l.Status == "moved" || l.Status == "cancelled" {
			continue
		}
		sum.Orders++
		sum.Packages += len(l.Packages)
		for _, p := range l.Packages {
			if p.Scanned {
				sum.Scanned++
			}
		}
		c := byCo[l.CompanyID]
		if c == nil {
			c = &ports.ManifestCompany{ID: l.CompanyID}
			byCo[l.CompanyID] = c
		}
		c.Orders++
		c.Packages += len(l.Packages)
	}
	for _, c := range byCo {
		if s.Catalog != nil {
			if name, err := s.Catalog.CompanyName(ctx, c.ID); err == nil {
				c.Name = name
			}
		}
		sum.Companies = append(sum.Companies, *c)
	}
	sort.Slice(sum.Companies, func(i, j int) bool { return sum.Companies[i].ID < sum.Companies[j].ID })
	m.Summary = sum
}

// dispatchIncomplete entrega al courier una orden con bultos faltantes ya
// autorizada y deja el registro y el aviso.
func (s *Service) dispatchIncomplete(ctx context.Context, id tenant.Identity, m ports.Manifest, l ports.ManifestLine, o domain.Order, reason, evidence string) error {
	from, err := o.Apply(domain.EvHandOver)
	if err != nil {
		return err
	}
	scanned, missing := splitPackages(l.Packages)
	det := IncompleteDispatch{Kind: "despacho_incompleto", ManifestID: m.ID, Courier: m.Courier, WaveID: o.WaveID,
		Reason: reason, AuthorizedBy: id.Actor(), PackagesTotal: len(l.Packages), ScannedPackages: scanned, MissingPackages: missing}
	text := fmt.Sprintf("%s · despachada con %d de %d bulto(s); faltan %s · autoriza %s · motivo: %s",
		evidence, len(scanned), len(l.Packages), strings.Join(missing, ", "), id.Name, reason)
	out, err := s.move(ctx, o, from, domain.EvHandOver, id.Actor(), text, ports.TransitionOpts{Details: det})
	if err != nil {
		return err
	}
	s.notices.OnIncompleteDispatch(ctx, out, det, id.Name)
	return nil
}
