package application

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/platform/xlsx"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// Textos del registro de acciones (Tarea 06, Fase 3; cambio de Jean): los mismos para la
// pantalla y para el Excel, así no se duplican en el front.

var activityActions = map[string]string{
	// órdenes
	"create": "Orden creada", "start_validation": "Validación iniciada", "validate": "Validada", "void": "Anulada (sin stock)",
	"wave_confirmed": "Ola confirmada por Invas", "pick_confirmed": "Preparación confirmada", "assign_wave": "Asignación a ola solicitada",
	"cancel": "Cancelada", "incident": "Pasó a Incidencia", "report_incident": "Incidencia reportada", "resolve": "Incidencia resuelta",
	"revalidate": "Anulación resuelta", "hand_over": "Entregada al courier", "edit": "Orden editada", "edit_revalidate": "Editada (vuelve a validar)",
	"prioritize": "Priorizada", "unprioritize": "Prioridad quitada", "no_wave": "Regla sin ola disponible", "retry": "Trabajo reencolado",
	"duplicate": "Reintento del canal (no se duplicó)", "attach": "Documento adjunto",
	// empaque
	"start_packing": "Tomó la orden", "release_packing": "Soltó la orden", "register": "Registró bultos", "packed": "Empaque verificado",
	"print_label": "Imprimió etiqueta", "label_ready": "Etiqueta disponible", "label_pending": "Etiqueta pendiente", "label_failed": "Etiqueta con error",
	// retiro y entrega
	"manifest_created": "Creó manifiesto", "manifest_closed": "Cerró manifiesto (entrega)", "dispatch_anyway": "Despachar igual",
	"add": "Agregó la orden al manifiesto", "manifest_leave": "La orden salió del manifiesto",
	// usuarios
	"created": "Usuario creado", "updated": "Usuario editado", "enabled": "Usuario activado", "disabled": "Usuario desactivado",
	"password_reset": "Contraseña temporal generada", "password_email_sent": "Correo de contraseña enviado",
	"linked": "Vinculado con Keycloak", "relinked": "Revinculado con Keycloak", "provisioned": "Alta en el primer ingreso",
	// empresas cliente (Tarea 07)
	"company_created": "Empresa creada", "company_updated": "Empresa editada",
	"company_enabled": "Empresa activada", "company_disabled": "Empresa desactivada",
	"profiles_imported": "Carga masiva de perfiles",
	// solicitudes de perfiles (Tarea 07)
	"request_created": "Solicitud de perfil creada", "request_approved": "Solicitud de perfil aprobada",
	"request_rejected": "Solicitud de perfil rechazada",
	// productos y kits (Tarea 09)
	"product_created": "Producto creado", "product_updated": "Producto editado",
	"product_enabled": "Producto activado", "product_disabled": "Producto desactivado",
	"products_imported": "Carga masiva de productos",
}

func actionLabel(a string) string {
	if l := activityActions[a]; l != "" {
		return l
	}
	return a
}

func statusLabel(s any) string {
	str, _ := s.(string)
	if str == "" {
		return ""
	}
	if l := domain.Labels[domain.Status(str)]; l != "" {
		return l
	}
	return str
}

// transportCompany - el front manda la empresa del transporte como primera línea de notes
// («Empresa del transporte: X»); el resto son observaciones. TODO(api): campo propio en /close.
func transportCompany(notes string) (company, observation string) {
	lines := strings.Split(strings.TrimSpace(notes), "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "Empresa del transporte:") {
		company = strings.TrimSpace(strings.TrimPrefix(lines[0], "Empresa del transporte:"))
		lines = lines[1:]
	}
	return company, strings.TrimSpace(strings.Join(lines, "\n"))
}

func listText(v any) string {
	switch x := v.(type) {
	case []any:
		parts := make([]string, 0, len(x))
		for _, p := range x {
			parts = append(parts, fmt.Sprint(p))
		}
		if len(parts) == 0 {
			return "—"
		}
		return strings.Join(parts, ", ")
	case bool:
		if x {
			return "Sí"
		}
		return "No"
	case nil:
		return "—"
	case float64:
		return fmt.Sprint(int64(x))
	case string:
		if strings.TrimSpace(x) == "" {
			return "—" // dato vacío (por ejemplo, una empresa sin teléfono)
		}
		return x
	default:
		return fmt.Sprint(x)
	}
}

func roleText(v any) string {
	if s, ok := v.(string); ok && RoleLabels[s] != "" {
		return RoleLabels[s]
	}
	return listText(v)
}

// detailLines - el antes y el después completo de la acción, una línea por dato.
// Los datos que ya tienen su propia columna (estado, courier, bultos…) no se repiten.
func detailLines(module string, d map[string]any) []string {
	labels := map[string]string{
		"name": "Nombre", "role": "Perfil", "permissions": "Permisos", "companies": "Empresas", "allCompanies": "Todas las empresas",
		"username": "Usuario", "withoutEmail": "Sin correo", "temporary": "Contraseña temporal", "source": "Origen", "reason": "Motivo",
		"waveId": "Ola", "manifestId": "Manifiesto", "authorizedBy": "Autorizó", "missingPackages": "Bultos faltantes",
		"scannedPackages": "Bultos escaneados", "packagesTotal": "Bultos de la orden",
		"taxId": "RUT", "email": "Correo", "phone": "Teléfono", "waveCode": "Ola", "wmsCode": "Código Invas",
		"active": "Estado", "jobTitle": "Cargo", "companyName": "Empresa", "created": "Perfiles creados", "rows": "Filas leídas", "failed": "Filas con error",
		"requestKind": "Tipo de solicitud", "edited": "Editada al aceptar",
	}
	if module == "products" {
		// El SKU y el tipo ya tienen su columna; la empresa también.
		for k, v := range map[string]string{"description": "Descripción", "components": "Composición", "waveCode": "Ola"} {
			labels[k] = v
		}
		delete(labels, "companyName")
		delete(labels, "reason") // tiene su columna
	}
	if module == "dispatch" {
		for k, v := range map[string]string{"pickerName": "Quien retira", "pickerId": "RUT", "vehiclePlate": "Patente", "notes": "Notas"} {
			labels[k] = v
		}
	}
	keys := make([]string, 0, len(d))
	for k := range d {
		if labels[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := []string{}
	for _, k := range keys {
		v := d[k]
		text := func(x any) string {
			if k == "role" {
				return roleText(x)
			}
			if k == "authorizedBy" {
				return ParseActor(fmt.Sprint(x), "").Name
			}
			if k == "manifestId" {
				return "#" + listText(x)
			}
			if k == "requestKind" {
				return requestKindText(fmt.Sprint(x))
			}
			if k == "active" {
				if b, ok := x.(bool); ok && b {
					return "Activa"
				}
				return "Inactiva"
			}
			return listText(x)
		}
		if change, ok := v.(map[string]any); ok {
			out = append(out, fmt.Sprintf("%s: %s → %s", labels[k], text(change["from"]), text(change["to"])))
			continue
		}
		out = append(out, labels[k]+": "+text(v))
	}
	return out
}

// presentActivity completa los textos de una acción ya filtrada (details seguro).
func presentActivity(it *ActivityItem, courierName string) {
	it.ActionLabel = actionLabel(it.Action)
	from, to := statusLabel(it.Details["fromStatus"]), statusLabel(it.Details["toStatus"])
	switch {
	case from != "" && to != "":
		it.StatusChange = from + " → " + to
	case to != "":
		it.StatusChange = to
	}
	it.CourierName = courierName
	if it.CourierName == "" {
		it.CourierName, _ = it.Details["courier"].(string)
	}
	if notes, ok := it.Details["notes"].(string); ok && it.Module == "dispatch" {
		it.TransportCompany, it.Observation = transportCompany(notes)
	}
	it.DetailLines = detailLines(it.Module, it.Details)
}

// Columnas del Excel por apartado (las mismas de la pantalla).
var activityExport = map[string]struct {
	sheet  string
	header []string
	row    func(it ActivityItem, at string) []any
}{
	"orders": {"Órdenes", []string{"Fecha y hora", "Actor", "Perfil", "Humano/Sistema", "Acción", "N° orden", "Cliente", "Estado anterior → nuevo", "Motivo u observación", "Detalle"},
		func(it ActivityItem, at string) []any {
			return []any{at, it.Actor.Name, RoleLabels[it.Actor.Role], kindText(it.Actor.Kind), it.ActionLabel, it.Entity.Label, it.CompanyName, it.StatusChange, it.Reason, strings.Join(it.DetailLines, " · ")}
		}},
	"packing": {"Empaque", []string{"Fecha y hora", "Actor", "Perfil", "Acción", "N° orden", "Cliente", "Bultos", "Código de etiqueta", "Observación"},
		func(it ActivityItem, at string) []any {
			return []any{at, it.Actor.Name, RoleLabels[it.Actor.Role], it.ActionLabel, it.Entity.Label, it.CompanyName, listText(it.Details["packages"]), listText(it.Details["labelCode"]), it.Reason}
		}},
	"dispatch": {"Retiro y entrega", []string{"Fecha y hora", "Actor", "Perfil", "Acción", "N° manifiesto", "Courier", "Órdenes", "Bultos", "Quien retira", "RUT", "Patente", "Empresa del transporte", "Observación"},
		func(it ActivityItem, at string) []any {
			manifest := it.Entity.Label
			if it.Entity.Type == "order" {
				manifest = "#" + listText(it.Details["manifestId"])
			}
			obs := it.Observation
			if obs == "" {
				obs = it.Reason
			}
			return []any{at, it.Actor.Name, RoleLabels[it.Actor.Role], it.ActionLabel, manifest, it.CourierName, listText(it.Details["orders"]), listText(it.Details["packages"]),
				listText(it.Details["pickerName"]), listText(it.Details["pickerId"]), listText(it.Details["vehiclePlate"]), it.TransportCompany, obs}
		}},
	"products": {"Productos", []string{"Fecha y hora", "Actor", "Perfil", "Acción", "SKU", "Tipo", "Cliente", "Motivo", "Detalle"},
		func(it ActivityItem, at string) []any {
			return []any{at, it.Actor.Name, RoleLabels[it.Actor.Role], it.ActionLabel, it.Entity.Label, productTypeText(it.Details["productType"]), it.CompanyName, it.Reason, strings.Join(it.DetailLines, " · ")}
		}},
	"users": {"Usuarios", []string{"Fecha y hora", "Actor", "Perfil", "Acción", "Usuario afectado", "Perfil afectado", "Detalle"},
		func(it ActivityItem, at string) []any {
			return []any{at, it.Actor.Name, RoleLabels[it.Actor.Role], it.ActionLabel, it.Entity.Label, roleText(it.Details["targetRole"]), strings.Join(it.DetailLines, " · ")}
		}},
}

// productTypeText: «Kit» o «Artículo» (Tarea 09).
func productTypeText(v any) string {
	switch v {
	case "kit":
		return "Kit"
	case "simple":
		return "Artículo"
	}
	return ""
}

func kindText(k string) string {
	if k == "system" {
		return "Sistema"
	}
	return "Humano"
}

// Export - Excel con lo filtrado del apartado (module obligatorio), con sus columnas.
func (a *Activity) Export(ctx context.Context, id tenant.Identity, f ports.ActivityFilter, w io.Writer) error {
	spec, ok := activityExport[f.Module]
	if !ok {
		return fail(domain.ErrInvalid, "Elige el apartado que quieres exportar: orders, packing, dispatch, users o products.")
	}
	f.Limit, f.Offset = MaxExportRows, 0
	items, total, err := a.list(ctx, id, f)
	if err != nil {
		return err
	}
	if total > MaxExportRows {
		return fail(domain.ErrInvalid, "El filtro tiene %d acciones: exporta como máximo %d (acota las fechas).", total, MaxExportRows)
	}
	loc, _ := time.LoadLocation("America/Santiago")
	if loc == nil {
		loc = time.Local
	}
	rows := make([][]any, 0, len(items))
	for _, it := range items {
		rows = append(rows, spec.row(it, it.OccurredAt.In(loc).Format("2006-01-02 15:04")))
	}
	return xlsx.Write(w, spec.sheet, spec.header, rows)
}

// requestKindText - tipo de solicitud de perfil en español (Tarea 07).
func requestKindText(k string) string {
	switch k {
	case "new_profile":
		return "Nuevo perfil"
	case "activation":
		return "Activación"
	case "deactivation":
		return "Desactivación"
	}
	return k
}
