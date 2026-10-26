// Package domain - perfiles de usuario, roles y permisos (RF-01, RF-02).
// La identidad (contraseña, sesión, correos) vive en Keycloak (PV-18); aquí
// solo están las reglas del perfil de Nexus.
package domain

import (
	"errors"
	"net/mail"
	"slices"
	"strings"
	"time"
)

var (
	ErrNotFound       = errors.New("usuario no encontrado")
	ErrEmailTaken     = errors.New("el correo ya está registrado")
	ErrInvalid        = errors.New("datos de usuario inválidos")
	ErrSelfChange     = errors.New("no puedes cambiar tu propio rol, permisos, empresas ni estado")
	ErrIdPUnavailable = errors.New("el proveedor de identidad (Keycloak) no está disponible; no se hicieron cambios")
)

// Roles base (roles de Keycloak) y permisos adicionales (en Nexus).
var (
	Roles         = []string{"admin", "jefe_operaciones", "analista", "empaque", "cliente"}
	InternalRoles = []string{"admin", "jefe_operaciones", "analista", "empaque"}
	Permissions   = []string{"despacho"} // «Despacho» es un permiso, no un perfil
)

type User struct {
	ID           int64
	KeycloakID   string
	Email        string
	Name         string
	Role         string
	Permissions  []string
	AllCompanies bool    // personal interno con acceso a todas las empresas
	Companies    []int64 // empresas autorizadas (si no es AllCompanies)
	Active       bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Draft son los datos que el administrador puede fijar.
type Draft struct {
	Email        string
	Name         string
	Role         string
	Permissions  []string
	AllCompanies bool
	Companies    []int64
}

// Normalize limpia y valida un borrador. Devuelve un mensaje legible.
func (d *Draft) Normalize() (string, bool) {
	d.Email = strings.ToLower(strings.TrimSpace(d.Email))
	d.Name = strings.TrimSpace(d.Name)
	if a, err := mail.ParseAddress(d.Email); err != nil || a.Address != d.Email {
		return "correo inválido", false
	}
	if d.Name == "" || len(d.Name) > 120 {
		return "el nombre es obligatorio (máx. 120 caracteres)", false
	}
	if !slices.Contains(Roles, d.Role) {
		return "perfil inválido", false
	}
	perms := []string{}
	for _, p := range d.Permissions {
		p = strings.TrimSpace(strings.ToLower(p))
		if !slices.Contains(Permissions, p) {
			return "permiso desconocido: " + p, false
		}
		if !slices.Contains(perms, p) {
			perms = append(perms, p)
		}
	}
	d.Permissions = perms
	if d.Role == "cliente" {
		if d.AllCompanies {
			return "un usuario cliente no puede ver todas las empresas", false
		}
		if len(d.Permissions) > 0 {
			return "un usuario cliente no puede tener el permiso Despacho", false
		}
	}
	if !d.AllCompanies && len(d.Companies) == 0 {
		return "asigna al menos una empresa o marca «todas las empresas» (solo personal interno)", false
	}
	if d.AllCompanies {
		d.Companies = []int64{}
	}
	slices.Sort(d.Companies)
	d.Companies = slices.Compact(d.Companies)
	return "", true
}

// Changes describe una edición parcial.
type Changes struct {
	Name         *string
	Role         *string
	Permissions  *[]string
	AllCompanies *bool
	Companies    *[]int64
}

// Apply devuelve el borrador resultante de aplicar los cambios al usuario.
func (u User) Apply(c Changes) Draft {
	d := Draft{Email: u.Email, Name: u.Name, Role: u.Role, Permissions: u.Permissions,
		AllCompanies: u.AllCompanies, Companies: u.Companies}
	if c.Name != nil {
		d.Name = *c.Name
	}
	if c.Role != nil {
		d.Role = *c.Role
	}
	if c.Permissions != nil {
		d.Permissions = *c.Permissions
	}
	if c.AllCompanies != nil {
		d.AllCompanies = *c.AllCompanies
	}
	if c.Companies != nil {
		d.Companies = *c.Companies
	}
	return d
}

// TouchesScope indica si el cambio amplía o reduce alcance (rol, permisos,
// empresas). Nadie puede hacerlo sobre sí mismo (H-02 escenario 3).
func (c Changes) TouchesScope() bool {
	return c.Role != nil || c.Permissions != nil || c.AllCompanies != nil || c.Companies != nil
}

type AuditEntry struct {
	ID        int64          `json:"id"`
	ActorID   *int64         `json:"actorId"`
	ActorName string         `json:"actorName"`
	Action    string         `json:"action"`
	Detail    map[string]any `json:"detail"`
	CreatedAt time.Time      `json:"createdAt"`
}

type ModuleAccess struct {
	Module  string   `json:"module"`
	Actions []string `json:"actions"`
}
