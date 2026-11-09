package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// Solicitudes de perfiles (Tarea 07, Fase 4; tabla profile_requests de Core 000011). El Cliente
// solicita un perfil nuevo, una activación o una desactivación para su empresa; las aprueba o
// rechaza quien tenga «Gestionar perfiles».

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
	CompanyID       int64
	CompanyName     string
	Kind            RequestKind
	TargetUserID    *int64
	ProfileName     string
	ProfileEmail    string
	ProfilePhone    string
	ProfileJobTitle string
	Reason          string
	Status          RequestStatus
	RequestedBy     int64
	RequestedByName string
	RequestedAt     time.Time
	ReviewedBy      *int64
	ReviewedByName  string
	ReviewedAt      *time.Time
	RejectionReason string
	CreatedUserID   *int64
}

// RequestDraft son los datos que envía el Cliente.
type RequestDraft struct {
	Kind         RequestKind
	TargetUserID int64 // activación o desactivación
	Name         string
	Email        string
	Phone        string
	JobTitle     string
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
	switch d.Kind {
	case RequestNewProfile:
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
	case RequestActivation, RequestDeactivation:
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
