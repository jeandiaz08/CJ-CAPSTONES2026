package domain

// Status - catálogo ACORDADO (DAS v1.1 PA-04, ERS §3.2.7).
type Status string

const (
	StatusPending     Status = "pending"      // Pendiente
	StatusValidating  Status = "validating"   // En validación (visible mientras Invas no responde)
	StatusValidated   Status = "validated"    // Validada (automática; antes "Autorizada")
	StatusPicking     Status = "picking"      // En preparación (Invas confirmó la ola)
	StatusPicked      Status = "picked"       // Preparada
	StatusPacking     Status = "packing"      // En empaque
	StatusReadyPickup Status = "ready_pickup" // Lista para retiro
	StatusHandedOver  Status = "handed_over"  // Entregada al courier

	// Excepciones
	StatusIncident  Status = "incident"  // Incidencia: vuelve al estado previo al resolverse
	StatusVoided    Status = "voided"    // Anulada: sin stock; visible y recuperable
	StatusCancelled Status = "cancelled" // Cancelada: terminal; oculta, no se elimina
)

var AllStatuses = []Status{StatusPending, StatusValidating, StatusValidated, StatusPicking, StatusPicked,
	StatusPacking, StatusReadyPickup, StatusHandedOver, StatusIncident, StatusVoided, StatusCancelled}

func ValidStatus(s string) bool {
	for _, x := range AllStatuses {
		if string(x) == s {
			return true
		}
	}
	return false
}

type Event string

const (
	EvStartValidation Event = "start_validation" // sistema
	EvValidate        Event = "validate"         // sistema: stock suficiente
	EvVoid            Event = "void"             // sistema: sin stock -> Anulada
	EvWaveConfirmed   Event = "wave_confirmed"   // Invas confirmó la asignación a ola
	EvPickConfirmed   Event = "pick_confirmed"   // Invas confirmó la preparación
	EvStartPacking    Event = "start_packing"    // Empaque toma la orden
	EvReleasePacking  Event = "release_packing"  // Empaque la devuelve a la bandeja
	EvPacked          Event = "packed"           // Empaque verificado (bultos registrados)
	EvHandOver        Event = "hand_over"        // Entrega física al courier (permiso Despacho)
	EvIncident        Event = "incident"         // Se detiene el avance
	EvResolve         Event = "resolve"          // Vuelve al estado previo a la incidencia
	EvRevalidate      Event = "revalidate"       // Anulada -> Pendiente («Resolver anulación»)
	EvCancel          Event = "cancel"           // Cancelada (terminal)
	EvEditRevalidate  Event = "edit_revalidate"  // Edición que obliga a validar de nuevo
)

// transitions: estado actual + evento -> siguiente estado.
// Todo lo que no esté aquí está PROHIBIDO (p. ej. pending -> handed_over).
// "resolve" se maneja aparte porque su destino es el estado previo.
var transitions = map[Status]map[Event]Status{
	StatusPending:     {EvStartValidation: StatusValidating, EvIncident: StatusIncident, EvCancel: StatusCancelled},
	StatusValidating:  {EvValidate: StatusValidated, EvVoid: StatusVoided, EvIncident: StatusIncident, EvCancel: StatusCancelled, EvEditRevalidate: StatusPending},
	StatusValidated:   {EvWaveConfirmed: StatusPicking, EvIncident: StatusIncident, EvCancel: StatusCancelled, EvEditRevalidate: StatusPending},
	StatusPicking:     {EvPickConfirmed: StatusPicked, EvIncident: StatusIncident},
	StatusPicked:      {EvStartPacking: StatusPacking, EvIncident: StatusIncident},
	StatusPacking:     {EvPacked: StatusReadyPickup, EvReleasePacking: StatusPicked, EvIncident: StatusIncident},
	StatusReadyPickup: {EvHandOver: StatusHandedOver, EvIncident: StatusIncident},
	StatusIncident:    {EvCancel: StatusCancelled},
	StatusVoided:      {EvRevalidate: StatusPending, EvCancel: StatusCancelled},
}

func NextStatus(from Status, ev Event) (Status, bool) {
	next, ok := transitions[from][ev]
	return next, ok
}

// Label en español para la interfaz y los registros.
var Labels = map[Status]string{
	StatusPending: "Pendiente", StatusValidating: "En validación", StatusValidated: "Validada",
	StatusPicking: "En preparación", StatusPicked: "Preparada", StatusPacking: "En empaque",
	StatusReadyPickup: "Lista para retiro", StatusHandedOver: "Entregada al courier",
	StatusIncident: "Incidencia", StatusVoided: "Anulada", StatusCancelled: "Cancelada",
}

// Tipos de incidencia (CU-03): el motivo concreto va en la nota.
const (
	IncidentData           = "datos"              // producto, comuna o dirección inválidos (H-07)
	IncidentInvas          = "invas"              // Invas no respondió / cola agotada
	IncidentPacking        = "diferencia_empaque" // faltante, dañado o no coincide (H-11/H-12)
	IncidentCourierCover   = "courier_cobertura"  // el courier no llega a la comuna (H-13)
	IncidentCourierPackage = "courier_bultos"     // el courier no admite varios bultos (Recíbelo)
	IncidentCancelRequest  = "cancelacion"        // cancelación de una orden que ya está en Invas
	IncidentDelivery       = "retiro"             // problema detectado al retirar
	IncidentIntegration    = "integracion"        // error de integración
	IncidentOther          = "otro"
)

var IncidentKinds = []string{IncidentData, IncidentInvas, IncidentPacking, IncidentCourierCover,
	IncidentCourierPackage, IncidentCancelRequest, IncidentDelivery, IncidentIntegration, IncidentOther}

var IncidentLabels = map[string]string{
	IncidentData: "Datos inconsistentes", IncidentInvas: "Invas no responde", IncidentPacking: "Diferencia en preparación o empaque",
	IncidentCourierCover: "Courier sin cobertura", IncidentCourierPackage: "Courier no admite los bultos",
	IncidentCancelRequest: "Cancelación por confirmar en Invas", IncidentDelivery: "Problema al retirar",
	IncidentIntegration: "Error de integración", IncidentOther: "Otro",
}
