package application

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
	"github.com/nexuswst/backend-core/internal/modules/clients/ports"
	"github.com/nexuswst/backend-core/internal/platform/xlsx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// Clientes › Administración clientes › Empresas (Tarea 07, RF-04, H-03). Quién puede hacer qué
// lo decide la ruta (consultar: Administrador o quien gestione perfiles; cambiar: permiso
// «Gestionar perfiles»). Cada cambio queda en user_audit con el antes, el después y el motivo.

// MaxExportRows - tope de filas del Excel de empresas.
const MaxExportRows = 10000

var ErrExportTooBig = errors.New("hay demasiadas empresas para exportar: acota los filtros")

func (s *Service) AdminList(ctx context.Context, f ports.CompanyFilter) ([]domain.Company, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.AdminList(ctx, f)
}

func (s *Service) Get(ctx context.Context, id int64) (domain.Company, error) {
	return s.repo.Get(ctx, id)
}

// Create da de alta una empresa. El código Invas solo lo fija el Administrador (canWMS).
func (s *Service) Create(ctx context.Context, actor tenant.Identity, d domain.Draft, canWMS bool) (domain.Company, error) {
	if !canWMS {
		d.WMSCode = ""
	}
	if errs := d.Normalize(true); len(errs) > 0 {
		return domain.Company{}, errs
	}
	return s.repo.Create(ctx, d, actor.UserID, ports.Audit{Action: "company_created", Detail: map[string]any{
		"name": d.Name, "taxId": d.TaxID, "email": d.Email, "phone": d.Phone, "waveCode": waveText(d.WaveCode), "wmsCode": d.WMSCode,
	}})
}

// Update guarda todos los campos del formulario. El dígito verificador del RUT se valida solo si
// el RUT cambió (las empresas antiguas pueden tener otro formato).
func (s *Service) Update(ctx context.Context, actor tenant.Identity, id int64, d domain.Draft, canWMS bool) (domain.Company, error) {
	cur, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Company{}, err
	}
	if !canWMS {
		d.WMSCode = cur.WMSCode
	}
	if errs := d.Normalize(strings.TrimSpace(d.TaxID) != cur.TaxID); len(errs) > 0 {
		return domain.Company{}, errs
	}
	changes := companyDiff(cur, d)
	if len(changes) == 0 {
		return cur, nil
	}
	if err := s.repo.Update(ctx, id, d, actor.UserID, ports.Audit{Action: "company_updated", Detail: changes}); err != nil {
		return domain.Company{}, err
	}
	return s.repo.Get(ctx, id)
}

// Disable deja la empresa inactiva: sus usuarios dejan de operar en su siguiente petición (el
// middleware responde 403 «company_disabled» a quien no tenga otra empresa activa) y el
// historial se conserva (H-03, criterio 3). El motivo es obligatorio.
func (s *Service) Disable(ctx context.Context, actor tenant.Identity, id int64, reason string) (domain.Company, error) {
	return s.setActive(ctx, actor, id, false, reason)
}

func (s *Service) Enable(ctx context.Context, actor tenant.Identity, id int64, reason string) (domain.Company, error) {
	return s.setActive(ctx, actor, id, true, reason)
}

func (s *Service) setActive(ctx context.Context, actor tenant.Identity, id int64, active bool, reason string) (domain.Company, error) {
	r, err := domain.NormalizeReason(reason, !active)
	if err != nil {
		return domain.Company{}, err
	}
	action := "company_enabled"
	if !active {
		action = "company_disabled"
	}
	detail := map[string]any{"active": map[string]any{"from": !active, "to": active}}
	if r != "" {
		detail["reason"] = r
	}
	if err := s.repo.SetActive(ctx, id, active, actor.UserID, ports.Audit{Action: action, Detail: detail}); err != nil {
		return domain.Company{}, err
	}
	return s.repo.Get(ctx, id)
}

// Export escribe el Excel «Empresas» con los mismos filtros de la tabla (o las seleccionadas).
// withWMS: el código Invas solo lo ve el Administrador.
func (s *Service) Export(ctx context.Context, f ports.CompanyFilter, withWMS bool, w io.Writer) error {
	f.Limit, f.Offset = MaxExportRows+1, 0
	items, _, err := s.repo.AdminList(ctx, f)
	if err != nil {
		return err
	}
	if len(items) > MaxExportRows {
		return ErrExportTooBig
	}
	header := []string{"Nombre empresa", "RUT", "Correo", "Teléfono", "Ola", "N° perfiles", "Estado", "Fecha de creación"}
	if withWMS {
		header = append(header, "Código Invas")
	}
	rows := make([][]any, 0, len(items))
	for _, c := range items {
		state := "Activa"
		if !c.Active {
			state = "Inactiva"
		}
		row := []any{c.Name, c.TaxID, c.Email, c.Phone, waveText(c.WaveCode), c.ProfilesCount, state,
			c.CreatedAt.In(chile).Format("02-01-2006 15:04")}
		if withWMS {
			row = append(row, c.WMSCode)
		}
		rows = append(rows, row)
	}
	return xlsx.Write(w, "Empresas", header, rows)
}

// chile: las fechas del Excel van en hora de Chile (si la imagen no trae tzdata, UTC-3).
var chile = func() *time.Location {
	if loc, err := time.LoadLocation("America/Santiago"); err == nil {
		return loc
	}
	return time.FixedZone("CLT", -3*3600)
}()

// GeneralWave es el texto de la ola NULL: «sin ola fija, se usan las reglas» (PV-19).
const GeneralWave = "General (según reglas)"

func waveText(code *string) string {
	if code == nil {
		return GeneralWave
	}
	return *code
}

func companyDiff(cur domain.Company, d domain.Draft) map[string]any {
	out := map[string]any{}
	change := func(key string, from, to string) {
		if from != to {
			out[key] = map[string]any{"from": from, "to": to}
		}
	}
	change("name", cur.Name, d.Name)
	change("taxId", cur.TaxID, d.TaxID)
	change("email", cur.Email, d.Email)
	change("phone", cur.Phone, d.Phone)
	change("waveCode", waveText(cur.WaveCode), waveText(d.WaveCode))
	change("wmsCode", cur.WMSCode, d.WMSCode)
	return out
}
