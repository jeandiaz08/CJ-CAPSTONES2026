package domain

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Perfiles de clientes (Tarea 07, Clientes › Administración clientes › Perfiles). El perfil es un
// usuario con rol Cliente, una sola empresa y sin permisos internos; además guarda teléfono y cargo.

// ProfileScope: «client» = perfiles con rol Cliente (esta tarea); «internal» = perfiles de
// WeStorage (Configuración › Usuarios WeStorage, tarea siguiente).
type ProfileScope string

const (
	ProfileScopeClient   ProfileScope = "client"
	ProfileScopeInternal ProfileScope = "internal"
)

// ProfileDraft son los datos del formulario «Nuevo perfil» y de la importación.
type ProfileDraft struct {
	Name      string
	CompanyID int64
	Email     string // con correo, Keycloak envía el enlace para definir la contraseña
	Username  string // «usuario sin correo»
	Phone     string
	JobTitle  string
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

// Normalize valida lo que exige la base (nombre obligatorio, correo o usuario, formato de usuario,
// largos de teléfono y cargo) y la empresa. withLogin: false al editar (correo y usuario no cambian).
func (d *ProfileDraft) Normalize(withLogin bool) ValidationErrors {
	var errs ValidationErrors
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }
	d.Name = strings.Join(strings.Fields(d.Name), " ")
	d.Email = strings.ToLower(strings.TrimSpace(d.Email))
	d.Username = strings.ToLower(strings.TrimSpace(d.Username))
	d.Phone = strings.TrimSpace(d.Phone)
	d.JobTitle = strings.Join(strings.Fields(d.JobTitle), " ")

	if d.Name == "" || utf8.RuneCountInString(d.Name) > 120 {
		add("name", "El nombre es obligatorio (máximo 120 caracteres).")
	}
	if d.CompanyID <= 0 {
		add("companyId", "Elige la empresa.")
	}
	if withLogin {
		switch {
		case d.Email != "" && d.Username != "":
			add("email", "Indica correo o nombre de usuario, no ambos.")
		case d.Email != "":
			if a, err := mail.ParseAddress(d.Email); err != nil || a.Address != d.Email {
				add("email", "Correo inválido.")
			}
		case d.Username == "":
			add("username", "Sin correo, el nombre de usuario es obligatorio.")
		case !ValidUsername(d.Username):
			add("username", "Nombre de usuario inválido: de 3 a 40 caracteres, solo minúsculas, números, punto, guion o guion bajo; empieza con letra o número.")
		}
	}
	if d.Phone != "" && (len(d.Phone) < 6 || len(d.Phone) > 30) {
		add("phone", "Teléfono inválido (entre 6 y 30 caracteres).")
	}
	if utf8.RuneCountInString(d.JobTitle) > 80 {
		add("jobTitle", "El cargo admite hasta 80 caracteres.")
	}
	return errs
}
