package domain

import (
	"net/mail"
	"slices"
	"strings"
	"unicode/utf8"
)

// Perfiles de clientes (Tarea 07, Clientes › Administración clientes › Perfiles). El perfil es un
// usuario con rol Cliente, una sola empresa y sin permisos internos; además guarda teléfono y cargo.

// ProfileScope: «client» = perfiles con rol Cliente (Tarea 07); «internal» = perfiles de
// WeStorage (Configuración › Usuarios WeStorage, Tarea 08).
type ProfileScope string

const (
	ProfileScopeClient   ProfileScope = "client"
	ProfileScopeInternal ProfileScope = "internal"
)

// ProfileRoles: roles que se ofrecen en Usuarios WeStorage. Analista, KAM y SAC son perfiles
// futuros (regla 11 de la matriz): no se ofrecen ni se listan.
var ProfileRoles = []string{"admin", "jefe_operaciones", "empaque"}

// RolePermissions: permisos que admite cada rol interno (Matriz v1.2). «Despacho» para Jefe de
// Operaciones y Empaque; «Gestionar perfiles» solo para el Administrador.
var RolePermissions = map[string][]string{
	"admin":            {"gestionar_perfiles"},
	"jefe_operaciones": {"despacho"},
	"empaque":          {"despacho"},
}

// ProfileDraft son los datos del formulario «Nuevo perfil» y de la importación. El perfil Cliente
// siempre tiene correo (decisión del 08-11-2026): Keycloak le envía el enlace para definir la
// contraseña. El usuario sin correo (Username) es solo para personal interno (SCRUM-231).
// Rol y permisos solo en el alcance interno; el Cliente tiene empresa y el interno no.
type ProfileDraft struct {
	Name        string
	CompanyID   int64
	Email       string
	Username    string
	Phone       string
	JobTitle    string
	Role        string
	Permissions []string
}

// FieldError y ValidationErrors: el 422 indica el motivo por campo.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors []FieldError

func (v ValidationErrors) Error() string {
	msgs := make([]string, 0, len(v))
	for _, f := range v {
		msgs = append(msgs, f.Message)
	}
	return strings.Join(msgs, " · ")
}

// Normalize valida nombre, empresa (Cliente), correo (obligatorio para el Cliente; el interno puede
// usar un nombre de usuario), rol y permisos (interno) y largos de teléfono y cargo.
// withLogin: false al editar (el correo y el usuario no cambian).
func (d *ProfileDraft) Normalize(scope ProfileScope, withLogin bool) ValidationErrors {
	var errs ValidationErrors
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }
	d.Name = strings.Join(strings.Fields(d.Name), " ")
	d.Email = strings.ToLower(strings.TrimSpace(d.Email))
	d.Username = strings.ToLower(strings.TrimSpace(d.Username))
	d.Phone = strings.TrimSpace(d.Phone)
	d.JobTitle = strings.Join(strings.Fields(d.JobTitle), " ")
	internal := scope == ProfileScopeInternal

	if d.Name == "" || utf8.RuneCountInString(d.Name) > 120 {
		add("name", "El nombre es obligatorio (máximo 120 caracteres).")
	}
	if !internal && d.CompanyID <= 0 {
		add("companyId", "Elige la empresa.")
	}
	if withLogin {
		switch {
		case internal && d.Email == "" && d.Username != "":
			if !ValidUsername(d.Username) {
				add("username", "Nombre de usuario inválido: de 3 a 40 caracteres, solo minúsculas, números, punto, guion o guion bajo, y debe empezar con letra o número.")
			}
		case internal && d.Email == "":
			add("email", "Indica el correo o marca «Usuario sin correo» e indica el nombre de usuario.")
		case d.Email == "":
			add("email", "El correo es obligatorio: el perfil recibe ahí el enlace para definir su contraseña.")
		default:
			if a, err := mail.ParseAddress(d.Email); err != nil || a.Address != d.Email {
				add("email", "Correo inválido.")
			}
			d.Username = "" // con correo, el usuario es el correo
		}
	}
	if internal {
		errs = append(errs, d.normalizeRole()...)
	}
	if d.Phone != "" && (len(d.Phone) < 6 || len(d.Phone) > 30) {
		add("phone", "Teléfono inválido (entre 6 y 30 caracteres).")
	}
	if utf8.RuneCountInString(d.JobTitle) > 80 {
		add("jobTitle", "El cargo admite hasta 80 caracteres.")
	}
	return errs
}

// normalizeRole valida el rol interno y deja solo los permisos que ese rol admite, sin repetir.
func (d *ProfileDraft) normalizeRole() ValidationErrors {
	allowed, ok := RolePermissions[d.Role]
	if !ok {
		return ValidationErrors{{Field: "role", Message: "Elige el rol: Administrador, Jefe de Operaciones o Empaque."}}
	}
	perms := []string{}
	for _, p := range d.Permissions {
		p = strings.ToLower(strings.TrimSpace(p))
		if !slices.Contains(allowed, p) {
			return ValidationErrors{{Field: "permissions", Message: "Ese permiso no corresponde al rol: Despacho es para Jefe de Operaciones y Empaque; Gestionar perfiles, solo para el Administrador."}}
		}
		if !slices.Contains(perms, p) {
			perms = append(perms, p)
		}
	}
	slices.Sort(perms)
	d.Permissions = perms
	return nil
}
