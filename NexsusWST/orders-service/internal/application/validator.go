package application

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

const (
	ActorValidator     = "system:validador"
	MaxValidationTries = 5
)

// Validator - validación automática (RF-12, CU-01, H-07) que ejecuta el Worker.
//
//  1. Datos: empresa activa, productos existentes y activos, courier activo,
//     comuna del catálogo (con sugerencia por similitud) y cobertura del
//     courier. Un problema de datos deja la orden en Incidencia con su motivo
//     y la acción de corrección; no avanza.
//  2. Stock: stock de Invas − comprometido de Nexus (DA-09), en transacción.
//     Sin stock -> Anulada (visible y recuperable) y aviso al cliente.
//  3. Si Invas no responde, la orden queda «En validación» con reintentos de
//     espera creciente; agotados, pasa a Incidencia. Nunca se informa validada.
type Validator struct {
	svc  *Service
	log  *slog.Logger
	base time.Duration
}

func NewValidator(svc *Service) *Validator {
	return &Validator{svc: svc, log: svc.Log.With("component", "validator"), base: 5 * time.Second}
}

// RunOnce procesa las órdenes vencidas (SELECT … FOR UPDATE SKIP LOCKED).
func (v *Validator) RunOnce(ctx context.Context) {
	orders, err := v.svc.Orders.ClaimDueValidations(ctx, 10, time.Minute)
	if err != nil {
		v.log.Warn("no se pudieron tomar órdenes para validar", "err", err)
		return
	}
	for _, o := range orders {
		v.validate(ctx, o)
	}
}

func (v *Validator) validate(ctx context.Context, o domain.Order) {
	log := v.log.With("order_id", o.ID)
	var err error
	if o.Status == domain.StatusPending {
		from, _ := o.Apply(domain.EvStartValidation)
		if o, err = v.svc.Orders.Transition(ctx, o, from, domain.EvStartValidation, ActorValidator, "", ports.TransitionOpts{}); err != nil {
			log.Warn("no se pudo pasar a En validación", "err", err)
			return
		}
		v.svc.Notifier.OrderChanged(ctx, o)
	}

	// 1) Datos (lectura directa de PostgreSQL; no depende del Core)
	problems, err := v.dataProblems(ctx, o)
	if err != nil {
		log.Warn("lectura de catálogos falló; se reintenta", "err", err)
		return // el lease vence y la orden se toma en la próxima pasada
	}
	if len(problems) > 0 {
		v.incident(ctx, o, domain.IncidentData, strings.Join(problems, " · "))
		return
	}

	// 2) Stock en Invas
	skus := make([]string, 0, len(o.Items))
	for _, it := range o.Items {
		skus = append(skus, it.SKU)
	}
	stock, err := v.svc.WMS.GetStock(ctx, o.CompanyID, skus)
	if err != nil {
		attempts := o.ValidationAttempts + 1
		if attempts >= MaxValidationTries {
			v.incident(ctx, o, domain.IncidentInvas, "Invas no respondió tras "+strconv.Itoa(attempts)+" intentos: "+err.Error())
			return
		}
		next := time.Now().Add(v.base * time.Duration(1<<attempts)) // espera creciente
		_ = v.svc.Orders.ScheduleValidationRetry(ctx, o.ID, attempts, next, err.Error())
		log.Info("Invas no responde; se reintentará", "attempt", attempts, "next", next)
		return
	}

	// 3) Stock − comprometido, en transacción (dos órdenes por la última unidad)
	o, err = v.svc.Orders.FinishValidation(ctx, o.ID, stock)
	if err != nil {
		log.Warn("no se pudo cerrar la validación", "err", err)
		return
	}
	log.Info("validación terminada", "status", o.Status)
	v.svc.after(ctx, o, o.Note) // Anulada -> aviso al cliente
}

// dataProblems devuelve los motivos legibles, con la corrección sugerida.
func (v *Validator) dataProblems(ctx context.Context, o domain.Order) ([]string, error) {
	var out []string
	cat := v.svc.Catalog

	active, err := cat.CompanyActive(ctx, o.CompanyID)
	if err != nil {
		return nil, err
	}
	if !active {
		out = append(out, "La empresa está deshabilitada: no se aceptan órdenes nuevas")
	}

	skus := make([]string, 0, len(o.Items))
	for _, it := range o.Items {
		skus = append(skus, it.SKU)
	}
	missing, err := cat.MissingSKUs(ctx, o.CompanyID, skus)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		out = append(out, "Productos inexistentes o inactivos en el catálogo del cliente: "+strings.Join(missing, ", ")+
			". Corrige el SKU o crea el producto")
	}

	if ok, err := cat.CourierActive(ctx, o.Courier); err != nil {
		return nil, err
	} else if !ok {
		out = append(out, fmt.Sprintf("El courier «%s» no existe o está inactivo. Elige otro courier", o.Courier))
	}

	communes, err := cat.Communes(ctx)
	if err != nil {
		return nil, err
	}
	official, suggestion := domain.MatchCommune(o.Commune, communes)
	switch {
	case official == "" && suggestion != "":
		out = append(out, fmt.Sprintf("Comuna «%s» no reconocida. ¿Quisiste decir «%s»? Corrige la comuna", o.Commune, suggestion))
	case official == "":
		out = append(out, fmt.Sprintf("Comuna «%s» no reconocida. Corrige la comuna del destino", o.Commune))
	}

	if official != "" && o.Courier != "" {
		cs, err := v.svc.Rules.CourierSettings(ctx, o.Courier)
		if err != nil {
			return nil, err
		}
		if !cs.CoversAll && !containsFold(cs.Communes, official) {
			out = append(out, fmt.Sprintf("El courier «%s» no tiene cobertura en %s. Cambia el courier o la dirección", o.Courier, official))
		}
	}
	return out, nil
}

func containsFold(list []string, s string) bool {
	n := domain.NormalizeName(s)
	for _, x := range list {
		if domain.NormalizeName(x) == n {
			return true
		}
	}
	return false
}

func (v *Validator) incident(ctx context.Context, o domain.Order, kind, reason string) {
	from, err := o.Incident(kind)
	if err != nil {
		return
	}
	if o, err = v.svc.Orders.Transition(ctx, o, from, domain.EvIncident, ActorValidator, reason, ports.TransitionOpts{}); err == nil {
		v.svc.after(ctx, o, reason)
	}
}
