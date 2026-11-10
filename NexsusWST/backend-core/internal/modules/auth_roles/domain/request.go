package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// Solicitudes de perfiles (tabla profile_requests de Core 000011 y 000012). Scope «client» (Tarea 07):
// el Cliente solicita un perfil nuevo, una activación o una desactivación para su empresa. Scope
// «internal» (Tarea 08): el Administrador sin «Gestionar perfiles» lo solicita para el personal de
// WeStorage, con rol y permisos propuestos. Las aprueba o rechaza quien tenga «Gestionar perfiles».

type RequestKind string

const (
	RequestNewProfile   RequestKind = "new_profile"
	RequestActivation   RequestKind = "activation"
	RequestDeactivation RequestKind = "deactivation"
)

type RequestStatus string

const (
	RequestPending  RequestStatus = "pending"
	RequestApproved RequestStatus = "approved"
	RequestRejected RequestStatus = "rejected"
)

var (
	ErrRequestPending  = errors.New("ya hay una solicitud pendiente para ese perfil y tipo")
	ErrRequestReviewed = errors.New("la solicitud ya fue respondida")
)

// KindLabel y StatusLabel: textos en español para la campana y el correo.
func KindLabel(k RequestKind) string {
	switch k {
	case RequestNewProfile:
		return "Nuevo perfil"
	case RequestActivation:
		return "Activación"
	case RequestDeactivation:
		return "Desactivación"
	}
	return string(k)
}

type ProfileRequest struct {
	ID              int64
	Scope           ProfileScope
	CompanyID       int64 // 0 en las internas
	CompanyName     string
	Kind            RequestKind
	TargetUserID    *int64
	ProfileName     string
	ProfileEmail    string
	ProfilePhone    string
	ProfileJobTitle string
	// Solo internas: usuario sin correo, rol y permisos propuestos.
	ProfileUsername    string
	ProfileRole        string
	ProfilePermissions []string
	Reason             string
	Status             RequestStatus
	RequestedBy        int64
	RequestedByName    string
	RequestedAt        time.Time
	ReviewedBy         *int64
	ReviewedByName     string
	ReviewedAt         *time.Time
	RejectionReason    string
	CreatedUserID      *int64
}

// RequestDraft son los datos que envía quien solicita (el Cliente o, en las internas, el
// Administrador sin «Gestionar perfiles»).
type RequestDraft struct {
	Scope        ProfileScope // vacío = client
	Kind         RequestKind
	TargetUserID int64 // activación o desactivación
	Name         string
	Email        string
	Username     string // internas: perfil sin correo
	Phone        string
	JobTitle     string
	Role         string   // internas: rol propuesto
	Permissions  []string // internas: permisos propuestos
	Reason       string
}

// Normalize valida lo que exige la tabla: tipo, perfil afectado solo si no es nuevo, nombre (y,
// para un perfil nuevo, correo: el Cliente siempre tiene correo), motivo obligatorio y largos.
func (d *RequestDraft) Normalize() ValidationErrors {
	var errs ValidationErrors
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }
	d.Name = strings.Join(strings.Fields(d.Name), " ")
	d.Email = strings.ToLower(strings.TrimSpace(d.Email))
	d.Phone = strings.TrimSpace(d.Phone)
	d.JobTitle = strings.Join(strings.Fields(d.JobTitle), " ")
	d.Reason = strings.TrimSpace(d.Reason)
	if d.Scope == "" {
		d.Scope = ProfileScopeClient
	}
	internal := d.Scope == ProfileScopeInternal
	switch {
	case d.Scope != ProfileScopeClient && !internal:
		add("scope", "Alcance inválido.")
	case internal && d.Kind == RequestNewProfile:
		// Mismas reglas que «Nuevo perfil» de WeStorage: correo o usuario, rol y permisos del rol.
		p := ProfileDraft{Name: d.Name, Email: d.Email, Username: d.Username, Phone: d.Phone, JobTitle: d.JobTitle, Role: d.Role, Permissions: d.Permissions}
		errs = append(errs, p.Normalize(ProfileScopeInternal, true)...)
		d.Name, d.Email, d.Username, d.Phone, d.JobTitle, d.Permissions = p.Name, p.Email, p.Username, p.Phone, p.JobTitle, p.Permissions
	case d.Kind == RequestNewProfile:
		if d.Name == "" || utf8.RuneCountInString(d.Name) > 120 {
			add("name", "El nombre es obligatorio (máximo 120 caracteres).")
		}
		if d.Email == "" {
			add("email", "El correo es obligatorio: el perfil recibe ahí el enlace para definir su contraseña.")
		} else if a, err := mail.ParseAddress(d.Email); err != nil || a.Address != d.Email {
			add("email", "Correo inválido.")
		}
		if d.Phone != "" && (len(d.Phone) < 6 || len(d.Phone) > 30) {
			add("phone", "Teléfono inválido (entre 6 y 30 caracteres).")
		}
		if utf8.RuneCountInString(d.JobTitle) > 80 {
			add("jobTitle", "El cargo admite hasta 80 caracteres.")
		}
	case d.Kind == RequestActivation, d.Kind == RequestDeactivation:
		if d.TargetUserID <= 0 {
			add("targetUserId", "Elige el perfil.")
		}
	default:
		add("kind", "Tipo de solicitud inválido.")
	}
	if n := utf8.RuneCountInString(d.Reason); n < 5 || n > 500 {
		add("reason", "Indica el motivo (entre 5 y 500 caracteres).")
	}
	return errs
}
