// Package ports - contratos que usa la aplicación de Órdenes (hexagonal).
// Los adaptadores (PostgreSQL, Redis, Invas, couriers, archivos, correo)
// implementan estas interfaces; las pruebas usan versiones falsas.
package ports

import (
	"context"
	"io"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
)

// Job es una intención de trabajo asíncrono (bandeja transaccional, DAS §5.4).
type Job struct {
	ID        string // identificador de operación (idempotencia)
	Type      string // assign_wave, process_webhook...
	CompanyID int64
	OrderID   int64
	Priority  bool
	Payload   map[string]any
}

// Vistas del listado.
const (
	ViewTray     = ""         // bandeja de trabajo: oculta Canceladas
	ViewRegistry = "registro" // Registro histórico: Entregadas al courier y Canceladas
	ViewAll      = "all"
)

// ListFilter: CompanyIDs nil = todas (solo personal interno).
type ListFilter struct {
	CompanyIDs   []int64
	Statuses     []string
	View         string
	Search       string // número, referencia o destinatario
	Courier      string
	Channel      string
	From, To     *time.Time
	OnlyPriority bool
	WithoutWave  bool
	IncidentKind string
	IDs          []int64 // exportar seleccionados
	Sort         string  // recent (defecto), oldest, priority
	Limit        int
	Offset       int
}

// EventRecord - historial de la orden (solo agregado, DAS §4.3).
type EventRecord struct {
	ID         int64     `json:"id"`
	Type       string    `json:"type"` // created, status_change, edit, priority, wave_requested, print, document...
	Event      string    `json:"event"`
	FromStatus string    `json:"fromStatus"`
	ToStatus   string    `json:"toStatus"`
	Actor      string    `json:"actor"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurredAt"`
}

// TransitionOpts - efectos que acompañan un cambio de estado en la MISMA transacción.
type TransitionOpts struct {
	ReleaseCommitted bool  // libera el stock comprometido de la orden
	ClearWave        bool  // quita la ola (p. ej. al volver a validar)
	Jobs             []Job // trabajos para la cola (outbox)
}

type OrderRepository interface {
	Create(ctx context.Context, o domain.Order, actor string, createdBy *int64) (domain.Order, error)
	Get(ctx context.Context, companyIDs []int64, id int64) (domain.Order, error)
	List(ctx context.Context, f ListFilter) ([]domain.Order, int, error)
	Summary(ctx context.Context, companyIDs []int64) (map[domain.Status]int, error)
	Events(ctx context.Context, orderID int64) ([]EventRecord, error)
	AddEvent(ctx context.Context, orderID int64, typ, ev, actor, reason string) error

	// Transition persiste estado y campos de control con verificación de
	// versión + historial + efectos, en UNA transacción. ErrConflict si otra
	// operación ganó (p. ej. dos personas confirman a la vez).
	Transition(ctx context.Context, o domain.Order, from domain.Status, ev domain.Event, actor, reason string, opts TransitionOpts) (domain.Order, error)
	// SaveEdit guarda datos editados (y opcionalmente un cambio de estado).
	SaveEdit(ctx context.Context, before, after domain.Order, diff []string, actor string, ev domain.Event, opts TransitionOpts) (domain.Order, error)
	SetPriority(ctx context.Context, o domain.Order, priority bool, actor, reason string) (domain.Order, error)
	// AssignWave marca la ola en órdenes Validadas sin ola y encola el envío a Invas.
	AssignWave(ctx context.Context, companyIDs []int64, orderIDs []int64, waveID, actor, reason string) ([]int64, error)
	// RetryFailedJobs vuelve a encolar los trabajos fallidos de la orden (reejecución autorizada).
	RetryFailedJobs(ctx context.Context, orderID int64) (int, error)

	// --- Validación automática (Worker) ---
	ClaimDueValidations(ctx context.Context, limit int, lease time.Duration) ([]domain.Order, error)
	ScheduleValidationRetry(ctx context.Context, orderID int64, attempts int, next time.Time, lastErr string) error
	// FinishValidation: en una transacción compara stock de Invas − comprometido;
	// si alcanza suma el comprometido y deja Validada; si no, Anulada.
	FinishValidation(ctx context.Context, orderID int64, invasStock map[string]int) (domain.Order, error)

	// --- Invas (Worker) ---
	CompleteWaveAssignment(ctx context.Context, orderID int64, wmsRef string) (domain.Order, error)
	OrdersInStatus(ctx context.Context, status domain.Status, limit int) ([]domain.Order, error)

	// --- Motor de reglas de ola ---
	RuleCandidates(ctx context.Context, limit int) ([]domain.Order, error)
	MarkRulesChecked(ctx context.Context, orderIDs []int64) error
}

// CatalogReader lee datos del Core DIRECTO de PostgreSQL (DA-12): Órdenes no
// llama al Core ni usa su caché.
type CatalogReader interface {
	MissingSKUs(ctx context.Context, companyID int64, skus []string) ([]string, error)
	KitSKUs(ctx context.Context, companyID int64, skus []string) ([]string, error)
	CompanyActive(ctx context.Context, companyID int64) (bool, error)
	CourierActive(ctx context.Context, code string) (bool, error)
	Communes(ctx context.Context) ([]string, error)
	CompanyName(ctx context.Context, companyID int64) (string, error)
	ClientEmails(ctx context.Context, companyID int64) ([]string, error)
}

// ---------------------------------------------------------------------------
// Reglas de olas y configuración de couriers (RF-17, CU-02)
// ---------------------------------------------------------------------------

type WaveRule struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Position     int       `json:"position"`
	Active       bool      `json:"active"`
	Courier      string    `json:"courier"`
	CompanyID    *int64    `json:"companyId"`
	Channel      string    `json:"channel"`
	DispatchType string    `json:"dispatchType"`
	Warehouse    string    `json:"warehouse"`
	WaveCode     string    `json:"waveCode"`
	UpdatedBy    string    `json:"updatedBy"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CourierSettings struct {
	Courier     string   `json:"courier"`
	ArrivalTime string   `json:"arrivalTime"` // HH:MM, corte del courier
	MaxPackages *int     `json:"maxPackages"`
	CoversAll   bool     `json:"coversAll"`
	Communes    []string `json:"communes"` // cobertura si CoversAll = false
}

type RuleRepository interface {
	List(ctx context.Context, onlyActive bool) ([]WaveRule, error)
	Get(ctx context.Context, id int64) (WaveRule, error)
	Save(ctx context.Context, r WaveRule, actor string) (WaveRule, error) // crea si ID == 0
	Delete(ctx context.Context, id int64) error
	Reorder(ctx context.Context, ids []int64, actor string) error
	ResetChecks(ctx context.Context) error // tras cambiar reglas se reevalúan las órdenes sin ola

	CourierSettings(ctx context.Context, courier string) (CourierSettings, error)
	ListCourierSettings(ctx context.Context) ([]CourierSettings, error)
	SaveCourierSettings(ctx context.Context, s CourierSettings, actor string) error
}

// ---------------------------------------------------------------------------
// Empaque, bultos, documentos y retiro (RF-18 a RF-24)
// ---------------------------------------------------------------------------

type Document struct {
	ID          int64     `json:"id"`
	OrderID     int64     `json:"orderId"`
	CompanyID   int64     `json:"-"`
	Kind        string    `json:"kind"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	SHA256      string    `json:"sha256"`
	StorageKey  string    `json:"-"`
	UploadedBy  string    `json:"uploadedBy"`
	UploadedAt  time.Time `json:"uploadedAt"`
}

type Manifest struct {
	ID           int64          `json:"id"`
	Courier      string         `json:"courier"`
	Status       string         `json:"status"`
	CreatedBy    string         `json:"createdBy"`
	CreatedAt    time.Time      `json:"createdAt"`
	ClosedBy     string         `json:"closedBy,omitempty"`
	ClosedAt     *time.Time     `json:"closedAt,omitempty"`
	PickerName   string         `json:"pickerName,omitempty"`
	PickerID     string         `json:"pickerId,omitempty"`
	VehiclePlate string         `json:"vehiclePlate,omitempty"`
	Notes        string         `json:"notes,omitempty"`
	Lines        []ManifestLine `json:"lines"`
}

type ManifestLine struct {
	OrderID       int64            `json:"orderId"`
	CompanyID     int64            `json:"companyId"`
	ExternalRef   string           `json:"externalRef"`
	RecipientName string           `json:"recipientName"`
	AddressLine   string           `json:"addressLine"`
	Commune       string           `json:"commune"`
	Status        string           `json:"status"` // included, moved, cancelled, pending_packages, delivered
	MovedTo       string           `json:"movedTo,omitempty"`
	Packages      []ManifestBundle `json:"packages"`
}

type ManifestBundle struct {
	ID      int64  `json:"id"`
	Code    string `json:"code"`
	Scanned bool   `json:"scanned"`
}

type CourierQueue struct {
	Courier  string `json:"courier"`
	Orders   int    `json:"orders"`
	Packages int    `json:"packages"`
}

type FulfillmentRepository interface {
	Packages(ctx context.Context, orderID int64) ([]domain.Package, error)
	ReplacePackages(ctx context.Context, orderID int64, p []domain.Package, actor string, userID int64) ([]domain.Package, error)
	PackingQueue(ctx context.Context, companyIDs []int64, userID int64) ([]domain.Order, error)

	AddDocument(ctx context.Context, d Document) (Document, error)
	Documents(ctx context.Context, orderID int64) ([]Document, error)
	Document(ctx context.Context, orderID, docID int64) (Document, error)

	ReadyByCourier(ctx context.Context, companyIDs []int64) ([]CourierQueue, error)
	CreateManifest(ctx context.Context, courier string, orderIDs []int64, actor string) (Manifest, error)
	Manifest(ctx context.Context, id int64) (Manifest, error)
	Manifests(ctx context.Context, status string, limit int) ([]Manifest, error)
	ScanPackage(ctx context.Context, manifestID int64, code, actor string) (ManifestLine, error)
	SetLineStatus(ctx context.Context, manifestID, orderID int64, status, movedTo string) error
	OpenManifestOf(ctx context.Context, orderID int64) (int64, error)
	CloseManifest(ctx context.Context, m Manifest, actor string) error

	SaveImport(ctx context.Context, companyID int64, filename string, total, created int, errs []RowError, actor string) (int64, error)
}

// RowError - resultado de una fila inválida en una carga masiva.
type RowError struct {
	Row     int    `json:"row"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Seguimiento, integraciones y avisos (RF-05, RF-25, RF-27)
// ---------------------------------------------------------------------------

type TrackingEvent struct {
	ID          int64     `json:"id"`
	OrderID     int64     `json:"orderId"`
	Provider    string    `json:"provider"`
	EventID     string    `json:"eventId"`
	StatusRaw   string    `json:"status"`
	Description string    `json:"description"`
	OccurredAt  time.Time `json:"occurredAt"`
	ReceivedAt  time.Time `json:"receivedAt"`
}

type Integration struct {
	ID             int64     `json:"id"`
	CompanyID      int64     `json:"companyId"`
	Provider       string    `json:"provider"`
	Kind           string    `json:"kind"` // channel | courier
	Channel        string    `json:"channel"`
	DefaultCourier string    `json:"defaultCourier"`
	Active         bool      `json:"active"`
	SecretEnc      string    `json:"-"`
	CreatedBy      string    `json:"createdBy"`
	CreatedAt      time.Time `json:"createdAt"`
}

type InboxItem struct {
	ID            int64
	Provider      string
	IntegrationID int64
	Payload       string
	Attempts      int
}

type IntegrationRepository interface {
	List(ctx context.Context, companyIDs []int64) ([]Integration, error)
	Get(ctx context.Context, id int64) (Integration, error)
	Save(ctx context.Context, i Integration) (Integration, error)
	SetActive(ctx context.Context, id int64, active bool) error

	// Inbox de webhooks: se registra y responde de inmediato; se procesa por la cola.
	Receive(ctx context.Context, provider string, integrationID int64, externalID, payload string) (id int64, duplicate bool, err error)
	InboxItem(ctx context.Context, id int64) (InboxItem, error)
	FinishInbox(ctx context.Context, id int64, status, errMsg string, orderID *int64) error

	AddTracking(ctx context.Context, e TrackingEvent) (inserted bool, err error)
	Tracking(ctx context.Context, orderID int64) ([]TrackingEvent, error)
	FindOrderByRef(ctx context.Context, companyID int64, ref string) (int64, error)
	FindOrderByCourierCode(ctx context.Context, courier, code string) (int64, error)
}

type Notification struct {
	ID        int64     `json:"id"`
	CompanyID int64     `json:"companyId"`
	Audience  string    `json:"audience"` // internal | client | all
	OrderID   *int64    `json:"orderId"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

type NotificationRepository interface {
	Add(ctx context.Context, n Notification) (Notification, error)
	List(ctx context.Context, companyIDs []int64, internal bool, userID int64, onlyUnread bool, limit int) ([]Notification, int, error)
	MarkRead(ctx context.Context, ids []int64, userID int64) error
}

// ---------------------------------------------------------------------------
// Salidas técnicas
// ---------------------------------------------------------------------------

// Notifier publica avisos de pantalla DESPUÉS de persistir (PA-05, Pub/Sub).
type Notifier interface {
	OrderChanged(ctx context.Context, o domain.Order)
	Notify(ctx context.Context, n Notification)
}

// Mailer envía avisos de negocio por correo (mismo SMTP de westorage.cl).
type Mailer interface {
	Send(ctx context.Context, to []string, subject, body string) error
}

// FileStorage guarda documentos adjuntos (volumen Docker; S3/Spaces después).
type FileStorage interface {
	Put(ctx context.Context, key string, r io.Reader) (size int64, err error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

type Wave struct {
	ID   string `json:"id"`
	Code string `json:"code"` // GEN, AM, PM... (lo usan las reglas)
	Name string `json:"name"`
	Type string `json:"type"`
}

// WMSGateway es el puerto hacia Invas (PA-01 Adapter).
type WMSGateway interface {
	GetStock(ctx context.Context, companyID int64, skus []string) (map[string]int, error)
	ListWaves(ctx context.Context) ([]Wave, error)
	AssignToWave(ctx context.Context, o domain.Order, waveID string) (wmsRef string, err error)
	// PickingDone consulta si Invas terminó la preparación de la orden.
	PickingDone(ctx context.Context, o domain.Order) (bool, error)
}

// CourierGateway - etiquetas del courier (PA-01, una estrategia por courier).
type CourierGateway interface {
	Label(ctx context.Context, o domain.Order, packages []domain.Package) (pdf []byte, trackingCode string, err error)
}

// ChannelGateway - consulta de detalle de una orden avisada por referencia
// (p. ej. Mercado Libre envía solo el recurso). Solo lectura.
type ChannelGateway interface {
	FetchOrder(ctx context.Context, integ Integration, resource string) (domain.Draft, error)
}
