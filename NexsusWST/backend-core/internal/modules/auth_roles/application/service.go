// Package application - casos de uso de sesión y usuarios (CU-09, CU-10).
//
// El inicio/cierre de sesión, el token y la recuperación de contraseña los
// atiende Keycloak. El Core:
//   - crea o vincula el perfil de Nexus en el primer ingreso;
//   - administra usuarios: identidad y rol en Keycloak (Admin API) y perfil,
//     permisos y empresas en PostgreSQL;
//   - registra quién hizo cada cambio.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/platform/auth"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

type Service struct {
	users    ports.UserRepository
	idp      ports.IdentityProvider
	profiles tenant.ProfileStore
	log      *slog.Logger
}

func NewService(users ports.UserRepository, idp ports.IdentityProvider, profiles tenant.ProfileStore, log *slog.Logger) *Service {
	return &Service{users: users, idp: idp, profiles: profiles, log: log}
}

type UserView struct {
	ID           int64    `json:"id"`
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Permissions  []string `json:"permissions"`
	Companies    []int64  `json:"companies"`
	AllCompanies bool     `json:"allCompanies"`
	Active       bool     `json:"active"`
	Linked       bool     `json:"linked"` // ya ingresó o fue creado por Nexus en Keycloak
	LastLoginAt  any      `json:"lastLoginAt"`
}

func View(u domain.User) UserView {
	v := UserView{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, Permissions: u.Permissions,
		Companies: u.Companies, AllCompanies: u.AllCompanies, Active: u.Active, Linked: u.KeycloakID != ""}
	if v.Permissions == nil {
		v.Permissions = []string{}
	}
	if v.Companies == nil {
		v.Companies = []int64{}
	}
	if u.LastLoginAt != nil {
		v.LastLoginAt = u.LastLoginAt
	}
	return v
}

// ---------------------------------------------------------------------------
// Sesión
// ---------------------------------------------------------------------------

// Provision implementa tenant.Provisioner: primer ingreso de una persona que
// existe en Keycloak pero aún no tiene perfil vinculado (CU-09, excepción 5).
//   - Si hay un perfil con su correo (verificado) sin vincular, se vincula.
//   - Si no, se crea un perfil con su rol base de Keycloak y SIN empresas
//     (no ve datos hasta que el administrador le asigne alcance).
func (s *Service) Provision(ctx context.Context, c auth.Claims) (tenant.Profile, error) {
	email := strings.ToLower(strings.TrimSpace(c.Email))
	if email == "" || !c.EmailVerified {
		return tenant.Profile{}, tenant.ErrProfileNotFound
	}
	u, err := s.users.GetByEmail(ctx, email)
	switch {
	case err == nil && u.KeycloakID == "":
		if err := s.users.LinkKeycloak(ctx, u.ID, c.Subject); err != nil {
			return tenant.Profile{}, err
		}
		_ = s.users.Audit(ctx, nil, u.ID, "linked", map[string]any{"keycloakId": c.Subject})
		s.log.Info("perfil vinculado a Keycloak en su primer ingreso", "user", u.ID, "email", email)
	case err == nil:
		// El correo ya está vinculado a OTRA cuenta de Keycloak. Solo se vuelve
		// a vincular si esa cuenta ya no existe (p. ej. se recreó el realm);
		// si existe, no se mezclan identidades.
		exists, xerr := s.idp.Exists(ctx, u.KeycloakID)
		if xerr != nil {
			return tenant.Profile{}, xerr
		}
		if exists {
			s.log.Warn("primer ingreso con correo ya vinculado a otra cuenta activa de Keycloak", "email", email)
			return tenant.Profile{}, tenant.ErrProfileNotFound
		}
		if err := s.users.RelinkKeycloak(ctx, u.ID, u.KeycloakID, c.Subject); err != nil {
			return tenant.Profile{}, err
		}
		_ = s.users.Audit(ctx, nil, u.ID, "relinked", map[string]any{"from": u.KeycloakID, "to": c.Subject})
		s.log.Warn("perfil vuelto a vincular: su cuenta anterior ya no existe en Keycloak", "user", u.ID, "email", email)
	case errors.Is(err, domain.ErrNotFound):
		role := c.BaseRole(domain.Roles...)
		if role == "" {
			role = "cliente"
		}
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = email
		}
		u, err = s.users.Insert(ctx, c.Subject, domain.Draft{Email: email, Name: name, Role: role,
			Permissions: []string{}, Companies: []int64{}}, nil)
		if errors.Is(err, domain.ErrEmailTaken) { // otra petición lo creó en paralelo
			break
		}
		if err != nil {
			return tenant.Profile{}, err
		}
		_ = s.users.Audit(ctx, nil, u.ID, "provisioned", map[string]any{"role": role, "source": "primer ingreso"})
		s.log.Info("perfil creado en el primer ingreso", "user", u.ID, "email", email, "role", role)
	default:
		return tenant.Profile{}, err
	}
	return s.profiles.ProfileByKeycloakID(ctx, c.Subject)
}

type Me struct {
	tenant.Profile
	Modules []domain.ModuleAccess `json:"modules"`
}

// Me devuelve el perfil y los módulos visibles (para armar el menú).
func (s *Service) Me(ctx context.Context, id tenant.Identity) (Me, error) {
	_ = s.users.TouchLogin(ctx, id.UserID)
	mods, err := s.users.Modules(ctx, id.Role)
	if err != nil {
		return Me{}, err
	}
	p := id.Profile
	if p.Companies == nil {
		p.Companies = []int64{}
	}
	return Me{Profile: p, Modules: mods}, nil
}

// ---------------------------------------------------------------------------
// Administración de usuarios (solo administradores; lo exige la ruta)
// ---------------------------------------------------------------------------

func (s *Service) List(ctx context.Context, f ports.ListFilter) ([]UserView, int, error) {
	list, total, err := s.users.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]UserView, 0, len(list))
	for _, u := range list {
		out = append(out, View(u))
	}
	return out, total, nil
}

func (s *Service) Get(ctx context.Context, id int64) (UserView, error) {
	u, err := s.users.GetByID(ctx, id)
	return View(u), err
}

func (s *Service) History(ctx context.Context, id int64) ([]domain.AuditEntry, error) {
	if _, err := s.users.GetByID(ctx, id); err != nil {
		return nil, err
	}
	return s.users.AuditOf(ctx, id, 200)
}

// ValidationError lleva el motivo legible de un rechazo.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return e.Msg }
func (e ValidationError) Unwrap() error { return domain.ErrInvalid }

func (s *Service) validate(ctx context.Context, d *domain.Draft) error {
	if msg, ok := d.Normalize(); !ok {
		return ValidationError{msg}
	}
	if len(d.Companies) > 0 {
		ok, err := s.users.CompaniesExist(ctx, d.Companies)
		if err != nil {
			return err
		}
		if !ok {
			return ValidationError{"alguna empresa indicada no existe o está deshabilitada"}
		}
	}
	return nil
}

// Create da de alta un usuario (CU-10): identidad y rol en Keycloak, perfil en
// Nexus y correo para definir la contraseña. Si algo falla a mitad, se deshace
// el alta en Keycloak para no dejar cambios a medias.
func (s *Service) Create(ctx context.Context, actor tenant.Identity, d domain.Draft) (UserView, error) {
	if err := s.validate(ctx, &d); err != nil {
		return UserView{}, err
	}
	if existing, err := s.users.GetByEmail(ctx, d.Email); err == nil {
		return UserView{}, fmt.Errorf("%w (usuario #%d)", domain.ErrEmailTaken, existing.ID)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return UserView{}, err
	}

	created := true
	kcID, err := s.idp.CreateUser(ctx, d.Email, d.Name)
	if errors.Is(err, domain.ErrEmailTaken) {
		// Existe en Keycloak pero no en Nexus (p. ej. creado en la consola y
		// nunca ingresó): se reutiliza y se le crea el perfil.
		kcID, err = s.idp.FindByEmail(ctx, d.Email)
		created = false
	}
	if err != nil {
		return UserView{}, err
	}
	undo := func() {
		if created {
			if e := s.idp.DeleteUser(context.WithoutCancel(ctx), kcID); e != nil {
				s.log.Error("no se pudo deshacer el alta en Keycloak", "keycloakId", kcID, "err", e)
			}
		}
	}
	if err := s.idp.SetRole(ctx, kcID, d.Role); err != nil {
		undo()
		return UserView{}, err
	}
	u, err := s.users.Insert(ctx, kcID, d, &actor.UserID)
	if err != nil {
		undo()
		return UserView{}, err
	}
	_ = s.users.Audit(ctx, &actor.UserID, u.ID, "created", map[string]any{
		"role": d.Role, "permissions": d.Permissions, "companies": d.Companies, "allCompanies": d.AllCompanies})

	if err := s.idp.SendSetPasswordEmail(ctx, kcID); err != nil {
		// El usuario ya existe: el correo se puede reenviar después.
		s.log.Warn("no se pudo enviar el correo para definir contraseña", "user", u.ID, "err", err)
	}
	return View(u), nil
}

// Update edita nombre, rol, permisos y empresas. Nadie puede cambiar su
// propio alcance (H-02 escenario 3).
func (s *Service) Update(ctx context.Context, actor tenant.Identity, id int64, c domain.Changes) (UserView, error) {
	if id == actor.UserID && c.TouchesScope() {
		return UserView{}, domain.ErrSelfChange
	}
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return UserView{}, err
	}
	d := u.Apply(c)
	if err := s.validate(ctx, &d); err != nil {
		return UserView{}, err
	}
	if u.KeycloakID != "" {
		if d.Name != u.Name {
			if err := s.idp.UpdateName(ctx, u.KeycloakID, d.Name); err != nil {
				return UserView{}, err
			}
		}
		if d.Role != u.Role {
			if err := s.idp.SetRole(ctx, u.KeycloakID, d.Role); err != nil {
				return UserView{}, err
			}
		}
	}
	if err := s.users.Update(ctx, id, d, actor.UserID); err != nil {
		return UserView{}, err
	}
	_ = s.users.Audit(ctx, &actor.UserID, id, "updated", diff(u, d))
	u, err = s.users.GetByID(ctx, id)
	return View(u), err
}

// Disable bloquea al usuario de inmediato: Keycloak lo deshabilita y cierra
// sus sesiones, y su perfil queda inactivo (Core y Órdenes lo leen en cada
// petición). El trabajo que tenía tomado vuelve a la bandeja: Órdenes ignora
// las tomas de usuarios inactivos. Su historial se conserva.
func (s *Service) Disable(ctx context.Context, actor tenant.Identity, id int64, reason string) (UserView, error) {
	return s.setActive(ctx, actor, id, false, reason)
}

func (s *Service) Enable(ctx context.Context, actor tenant.Identity, id int64, reason string) (UserView, error) {
	return s.setActive(ctx, actor, id, true, reason)
}

func (s *Service) setActive(ctx context.Context, actor tenant.Identity, id int64, active bool, reason string) (UserView, error) {
	if id == actor.UserID {
		return UserView{}, domain.ErrSelfChange
	}
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return UserView{}, err
	}
	if u.KeycloakID != "" {
		if err := s.idp.SetEnabled(ctx, u.KeycloakID, active); err != nil {
			return UserView{}, err
		}
		if !active {
			if err := s.idp.Logout(ctx, u.KeycloakID); err != nil {
				s.log.Warn("no se pudieron cerrar las sesiones (el perfil inactivo igual lo bloquea)", "user", id, "err", err)
			}
		}
	}
	if err := s.users.SetActive(ctx, id, active, actor.UserID); err != nil {
		return UserView{}, err
	}
	action := "enabled"
	if !active {
		action = "disabled"
	}
	_ = s.users.Audit(ctx, &actor.UserID, id, action, map[string]any{"reason": strings.TrimSpace(reason)})
	u.Active = active
	return View(u), nil
}

// ResendSetPassword vuelve a enviar el correo para definir la contraseña.
func (s *Service) ResendSetPassword(ctx context.Context, actor tenant.Identity, id int64) error {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if u.KeycloakID == "" {
		return ValidationError{"el usuario aún no está vinculado a Keycloak"}
	}
	if !u.Active {
		return ValidationError{"el usuario está deshabilitado"}
	}
	if err := s.idp.SendSetPasswordEmail(ctx, u.KeycloakID); err != nil {
		return err
	}
	_ = s.users.Audit(ctx, &actor.UserID, id, "password_email_sent", map[string]any{})
	return nil
}

func diff(u domain.User, d domain.Draft) map[string]any {
	out := map[string]any{}
	if u.Name != d.Name {
		out["name"] = map[string]any{"from": u.Name, "to": d.Name}
	}
	if u.Role != d.Role {
		out["role"] = map[string]any{"from": u.Role, "to": d.Role}
	}
	if !slices.Equal(u.Permissions, d.Permissions) {
		out["permissions"] = map[string]any{"from": u.Permissions, "to": d.Permissions}
	}
	if u.AllCompanies != d.AllCompanies || !slices.Equal(u.Companies, d.Companies) {
		out["companies"] = map[string]any{"from": u.Companies, "to": d.Companies, "allCompanies": d.AllCompanies}
	}
	return out
}
