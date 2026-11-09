package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/platform/events"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// Requests - solicitudes de perfiles (Tarea 07, Fase 4). El Cliente solicita para su empresa; quien
// tenga «Gestionar perfiles» aprueba (crea el perfil o aplica la activación o desactivación) o
// rechaza con motivo. Cada paso queda en user_audit, avisa por la campana y por correo. El correo
// y la campana nunca bloquean: si fallan, la solicitud igual se guarda y el error queda en el log.
type Requests struct {
	profiles *Profiles
	repo     ports.RequestRepository
	notify   Notifier
	mail     Mailer
	appURL   string
	log      *slog.Logger
}

type Notifier interface {
	Notify(ctx context.Context, n events.Notice)
}

type Mailer interface {
	Send(ctx context.Context, to []string, subject, body string) error
}

func NewRequests(profiles *Profiles, repo ports.RequestRepository, notify Notifier, mail Mailer, appURL string, log *slog.Logger) *Requests {
	return &Requests{profiles: profiles, repo: repo, notify: notify, mail: mail, appURL: strings.TrimRight(appURL, "/"), log: log}
}

var (
	ErrNoCompany        = errors.New("tu usuario no tiene una empresa activa")
	ErrRejectReason     = ValidationError{"indica el motivo del rechazo (entre 5 y 500 caracteres)"}
	ErrAlreadyInState   = errors.New("el perfil ya está en el estado que se pide: rechaza la solicitud")
	ErrEditOnlyNew      = errors.New("solo una solicitud de perfil nuevo se puede editar antes de aceptarla")
	errTargetOtherScope = domain.ErrNotFound
)

// RequestView es la solicitud que ve la pantalla.
type RequestView struct {
	ID              int64            `json:"id"`
	CompanyID       int64            `json:"companyId"`
	CompanyName     string           `json:"companyName"`
	Kind            string           `json:"kind"`
	KindLabel       string           `json:"kindLabel"`
	TargetUserID    *int64           `json:"targetUserId"`
	Profile         RequestedProfile `json:"profile"`
	Reason          string           `json:"reason"`
	Status          string           `json:"status"`
	RequestedBy     Person           `json:"requestedBy"`
	RequestedAt     time.Time        `json:"requestedAt"`
	ReviewedBy      *Person          `json:"reviewedBy"`
	ReviewedAt      *time.Time       `json:"reviewedAt"`
	RejectionReason string           `json:"rejectionReason"`
	CreatedUserID   *int64           `json:"createdUserId"`
}

type RequestedProfile struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	JobTitle string `json:"jobTitle"`
}

type Person struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func requestView(r domain.ProfileRequest) RequestView {
	v := RequestView{ID: r.ID, CompanyID: r.CompanyID, CompanyName: r.CompanyName, Kind: string(r.Kind), KindLabel: domain.KindLabel(r.Kind),
		TargetUserID: r.TargetUserID, Profile: RequestedProfile{Name: r.ProfileName, Email: r.ProfileEmail, Phone: r.ProfilePhone, JobTitle: r.ProfileJobTitle},
		Reason: r.Reason, Status: string(r.Status), RequestedBy: Person{ID: r.RequestedBy, Name: r.RequestedByName}, RequestedAt: r.RequestedAt,
		ReviewedAt: r.ReviewedAt, RejectionReason: r.RejectionReason, CreatedUserID: r.CreatedUserID}
	if r.ReviewedBy != nil {
		v.ReviewedBy = &Person{ID: *r.ReviewedBy, Name: r.ReviewedByName}
	}
	return v
}

// clientCompany: la empresa del Cliente (una; la primera activa).
func clientCompany(id tenant.Identity) (int64, error) {
	if !id.HasRole(tenant.RoleCliente) || len(id.Companies) == 0 {
		return 0, ErrNoCompany
	}
	return id.Companies[0], nil
}

// Create: el Cliente solicita un perfil nuevo, una activación o una desactivación para su empresa.
func (s *Requests) Create(ctx context.Context, actor tenant.Identity, d domain.RequestDraft) (RequestView, error) {
	companyID, err := clientCompany(actor)
	if err != nil {
		return RequestView{}, err
	}
	if errs := d.Normalize(); len(errs) > 0 {
		return RequestView{}, errs
	}
	var target *int64
	switch d.Kind {
	case domain.RequestNewProfile:
		if _, err := s.profiles.svc.users.GetByEmail(ctx, d.Email); err == nil {
			return RequestView{}, conflictErr{domain.ValidationErrors{{Field: "email", Message: "Ese correo ya tiene un perfil."}}}
		}
	default:
		p, err := s.profiles.get(ctx, domain.ProfileScopeClient, d.TargetUserID)
		if err != nil || p.CompanyID != companyID {
			return RequestView{}, errTargetOtherScope // un perfil de otra empresa no existe para este Cliente
		}
		if (d.Kind == domain.RequestActivation) == p.Active {
			state := "activo"
			if !p.Active {
				state = "inactivo"
			}
			return RequestView{}, ValidationError{fmt.Sprintf("el perfil ya está %s", state)}
		}
		d.Name, d.Email, d.Phone, d.JobTitle = p.Name, "", "", "" // se leen del perfil
		target = &p.ID
	}
	id, err := s.repo.Create(ctx, companyID, actor.UserID, d)
	if err != nil {
		return RequestView{}, err
	}
	_ = s.repo.AuditRequest(ctx, actor.UserID, id, target, "request_created", map[string]any{"requestKind": string(d.Kind), "reason": d.Reason, "name": d.Name})
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return RequestView{}, err
	}
	s.notify.Notify(ctx, events.Notice{ID: fmt.Sprintf("pr-%d-created", r.ID), CompanyID: r.CompanyID, Audience: "internal", Kind: "profile_request",
		Title: "Nueva solicitud de perfil", Body: fmt.Sprintf("%s · %s · %s", r.CompanyName, domain.KindLabel(r.Kind), r.ProfileName), RequestID: r.ID})
	s.mailManagers(r)
	return requestView(r), nil
}

// List: el Cliente ve las de su empresa (lo decide el servidor); Administración clientes, todas.
func (s *Requests) List(ctx context.Context, actor tenant.Identity, f ports.RequestFilter) ([]RequestView, int, ports.RequestCounts, error) {
	if actor.HasRole(tenant.RoleCliente) {
		f.CompanyIDs = actor.Companies
		if f.CompanyIDs == nil {
			f.CompanyIDs = []int64{}
		}
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 20
	}
	f.Offset = max(f.Offset, 0)
	list, total, counts, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, 0, counts, err
	}
	out := make([]RequestView, 0, len(list))
	for _, r := range list {
		out = append(out, requestView(r))
	}
	return out, total, counts, nil
}

func (s *Requests) Get(ctx context.Context, actor tenant.Identity, id int64) (RequestView, error) {
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return RequestView{}, err
	}
	if actor.HasRole(tenant.RoleCliente) && !actor.CanAccess(r.CompanyID) {
		return RequestView{}, domain.ErrNotFound
	}
	return requestView(r), nil
}

// PendingFor: solicitudes pendientes por perfil (el Cliente ve «Solicitud pendiente»).
func (s *Requests) PendingFor(ctx context.Context, userIDs []int64) (map[int64][]domain.RequestKind, error) {
	return s.repo.PendingFor(ctx, userIDs)
}

// Approve aplica la solicitud: un perfil nuevo se crea con los datos (o con los editados en
// «Editar y aceptar»); una activación o desactivación se aplica al perfil con el motivo pedido.
func (s *Requests) Approve(ctx context.Context, actor tenant.Identity, id int64, edits *domain.ProfileDraft) (RequestView, error) {
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return RequestView{}, err
	}
	if r.Status != domain.RequestPending {
		return RequestView{}, domain.ErrRequestReviewed
	}
	var created, target *int64
	detail := map[string]any{"requestKind": string(r.Kind), "name": r.ProfileName}
	switch r.Kind {
	case domain.RequestNewProfile:
		d := domain.ProfileDraft{Name: r.ProfileName, CompanyID: r.CompanyID, Email: r.ProfileEmail, Phone: r.ProfilePhone, JobTitle: r.ProfileJobTitle}
		if edits != nil {
			d = *edits
			d.CompanyID = r.CompanyID // la solicitud es de una empresa: no se cambia al aceptar
			detail["edited"] = true
			detail["name"] = d.Name
		}
		p, err := s.profiles.Create(ctx, actor, domain.ProfileScopeClient, d)
		if err != nil {
			return RequestView{}, err
		}
		created, target = &p.ID, &p.ID
	default:
		if edits != nil {
			return RequestView{}, ErrEditOnlyNew
		}
		active := r.Kind == domain.RequestActivation
		reason := fmt.Sprintf("Solicitud #%d: %s", r.ID, r.Reason)
		if utf8.RuneCountInString(reason) > 500 {
			reason = string([]rune(reason)[:500])
		}
		if _, err := s.profiles.SetActive(ctx, actor, domain.ProfileScopeClient, *r.TargetUserID, active, reason); err != nil {
			if errors.Is(err, ErrSameState) {
				return RequestView{}, ErrAlreadyInState
			}
			return RequestView{}, err
		}
		target = r.TargetUserID
	}
	if err := s.repo.Approve(ctx, r.ID, actor.UserID, created); err != nil {
		return RequestView{}, err
	}
	_ = s.repo.AuditRequest(ctx, actor.UserID, r.ID, target, "request_approved", detail)
	return s.answered(ctx, r.ID)
}

// Reject rechaza con motivo obligatorio.
func (s *Requests) Reject(ctx context.Context, actor tenant.Identity, id int64, reason string) (RequestView, error) {
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < 5 || n > 500 {
		return RequestView{}, ErrRejectReason
	}
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return RequestView{}, err
	}
	if err := s.repo.Reject(ctx, r.ID, actor.UserID, reason); err != nil {
		return RequestView{}, err
	}
	_ = s.repo.AuditRequest(ctx, actor.UserID, r.ID, r.TargetUserID, "request_rejected",
		map[string]any{"requestKind": string(r.Kind), "name": r.ProfileName, "reason": reason})
	return s.answered(ctx, r.ID)
}

// answered avisa al Cliente que la pidió (campana y correo) y devuelve la solicitud respondida.
func (s *Requests) answered(ctx context.Context, id int64) (RequestView, error) {
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return RequestView{}, err
	}
	verb := "aprobada"
	if r.Status == domain.RequestRejected {
		verb = "rechazada"
	}
	s.notify.Notify(ctx, events.Notice{ID: fmt.Sprintf("pr-%d-%s", r.ID, r.Status), CompanyID: r.CompanyID, Audience: "client", Kind: "profile_request_reviewed",
		Title: "Tu solicitud fue " + verb, Body: fmt.Sprintf("%s · %s", domain.KindLabel(r.Kind), r.ProfileName), RequestID: r.ID})
	s.mailRequester(r, verb)
	return requestView(r), nil
}

// ---------------------------------------------------------------------------
// Correo (no bloquea: se envía aparte y el error queda en el log)
// ---------------------------------------------------------------------------

func (s *Requests) send(to []string, subject, body string) {
	if s.mail == nil || len(to) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.mail.Send(ctx, to, subject, body); err != nil {
			s.log.Error("no se pudo enviar el correo de la solicitud de perfil", "subject", subject, "err", err)
		}
	}()
}

func (s *Requests) mailManagers(r domain.ProfileRequest) {
	to, err := s.repo.ManagerEmails(context.Background())
	if err != nil {
		s.log.Error("no se pudieron leer los destinatarios de la solicitud de perfil", "request", r.ID, "err", err)
		return
	}
	body := fmt.Sprintf("Hay una nueva solicitud de perfil en NexusWST.\n\nEmpresa: %s\nTipo: %s\nNombre del perfil: %s\nMotivo: %s\n\nRevísala en Clientes › Administración clientes › Solicitudes:\n%s/clientes/administracion?tab=solicitudes&solicitud=%d\n",
		r.CompanyName, domain.KindLabel(r.Kind), r.ProfileName, r.Reason, s.appURL, r.ID)
	s.send(to, fmt.Sprintf("[NexusWST] Nueva solicitud de perfil · %s", r.CompanyName), body)
}

func (s *Requests) mailRequester(r domain.ProfileRequest, verb string) {
	u, err := s.profiles.svc.users.GetByID(context.Background(), r.RequestedBy)
	if err != nil || !u.HasEmail() {
		return
	}
	body := fmt.Sprintf("Tu solicitud de perfil fue %s.\n\nEmpresa: %s\nTipo: %s\nNombre del perfil: %s\nMotivo: %s\n", verb, r.CompanyName,
		domain.KindLabel(r.Kind), r.ProfileName, r.Reason)
	if r.Status == domain.RequestRejected {
		body += fmt.Sprintf("Motivo del rechazo: %s\n", r.RejectionReason)
	}
	body += fmt.Sprintf("\nRevísala en Clientes › Perfiles empresa › Mis solicitudes:\n%s/clientes/perfiles-empresa?tab=solicitudes&solicitud=%d\n", s.appURL, r.ID)
	s.send([]string{u.Email}, fmt.Sprintf("[NexusWST] Tu solicitud de perfil fue %s", verb), body)
}
