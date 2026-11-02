"use client";

import { useState } from "react";
import { Ban, MoreVertical, RotateCcw, Star } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { DropdownMenu, type DropdownItem } from "@/components/DropdownMenu";
import { FormModal } from "@/components/FormModal";
import { showToast } from "@/components/Toast";
import { apiErrorMessage } from "@/services/api";
import { cancelOrder, recoverVoided, setPriority, type Order } from "@/services/orders";
import type { Role } from "@/state/auth";
import { canOperate, cancelBlockReason, priorityBlockReason, retryBlockReason } from "../rules";

type Kind = "priority" | "cancel" | "retry";
type Issue = { ref: string; reason: string };
type Outcome = { kind: Kind; ok: number; pendingConfirm: number; failed: Issue[]; skipped: Issue[] };

const MAX_PARALLEL = 5; // sin endpoints masivos: una petición por orden, por tandas

const SUCCESS_TEXT: Record<Kind, string> = { priority: "enviadas primero a la cola", cancel: "canceladas", retry: "enviadas a validar otra vez" };

// «Acciones (N)»: operan sobre las filas marcadas. Las que no cumplen el estado se omiten y se informan.
export function BulkActions({ selected, role, onDone }: { selected: Order[]; role: Role; onDone: () => void }) {
  const [kind, setKind] = useState<Kind | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const blockReason = (order: Order, action: Kind) => action === "cancel" ? cancelBlockReason(order, role) : action === "priority" ? priorityBlockReason(order) : retryBlockReason(order);
  const eligible = kind ? selected.filter((order) => !blockReason(order, kind)) : [];
  const skipped: Issue[] = kind ? selected.flatMap((order) => {
    const reason = blockReason(order, kind);
    return reason ? [{ ref: order.externalRef, reason }] : [];
  }) : [];

  const run = async (action: Kind, reason: string) => {
    const failed: Issue[] = [];
    let ok = 0, pendingConfirm = 0;
    const call = (order: Order) => action === "cancel" ? cancelOrder(order.id, reason) : action === "priority" ? setPriority(order.id, true, reason) : recoverVoided(order.id, reason);
    for (let i = 0; i < eligible.length; i += MAX_PARALLEL) {
      const batch = eligible.slice(i, i + MAX_PARALLEL);
      const results = await Promise.allSettled(batch.map(call));
      results.forEach((result, index) => {
        if (result.status === "rejected") failed.push({ ref: batch[index].externalRef, reason: apiErrorMessage(result.reason) });
        else if (action === "cancel" && result.value.status !== "cancelled") pendingConfirm++;
        else ok++;
      });
    }
    const done = ok + pendingConfirm;
    if (done > 0) showToast(`${done} de ${selected.length} ${done === 1 ? "orden procesada" : "órdenes procesadas"}.`, failed.length || skipped.length ? "warn" : "ok");
    setOutcome({ kind: action, ok, pendingConfirm, failed, skipped });
    setKind(null);
    onDone();
  };

  const items: DropdownItem[] = [
    ...(canOperate(role) ? [{ label: "Enviar primero a la cola", icon: <Star size={18} aria-hidden="true" />, onSelect: () => setKind("priority") }] : []),
    { label: "Cancelar seleccionadas", icon: <Ban size={18} aria-hidden="true" />, danger: true, onSelect: () => setKind("cancel") },
    ...(canOperate(role) ? [{ label: "Reintentar validación", icon: <RotateCcw size={18} aria-hidden="true" />, onSelect: () => setKind("retry") }] : []),
  ];

  const summary = (
    <div className="bulk-summary">
      <p><strong>{eligible.length}</strong> de {selected.length} {selected.length === 1 ? "orden seleccionada" : "órdenes seleccionadas"} se {eligible.length === 1 ? "procesará" : "procesarán"}.</p>
      {skipped.length > 0 && <IssueList title={`Se omiten ${skipped.length}:`} issues={skipped} />}
    </div>
  );

  return (
    <>
      <DropdownMenu
        className="bulk-actions"
        disabled={selected.length === 0}
        trigger={<><MoreVertical size={18} aria-hidden="true" />Acciones{selected.length ? ` (${selected.length})` : ""}</>}
        items={items}
      />
      {kind === "priority" && (
        <ConfirmDialog title="Enviar primero a la cola" reason="optional" reasonLabel="Motivo (140 máx.)" confirmLabel="Enviar primero" confirmDisabled={!eligible.length} onClose={() => setKind(null)} onConfirm={(reason) => run("priority", reason)}>{summary}</ConfirmDialog>
      )}
      {kind === "cancel" && (
        <ConfirmDialog
          title="Cancelar seleccionadas"
          reason="required"
          reasonLabel="Motivo de la cancelación (140 máx.)"
          reasonRequiredMessage="Indica el motivo de la cancelación."
          confirmLabel="Continuar"
          destructive
          confirmDisabled={!eligible.length}
          secondStep={{ message: `¿Seguro? ${eligible.length === 1 ? "La orden quedará Cancelada y pasará" : `Las ${eligible.length} órdenes quedarán Canceladas y pasarán`} al Registro histórico. No se puede deshacer.`, confirmLabel: "Sí, cancelar" }}
          onClose={() => setKind(null)}
          onConfirm={(reason) => run("cancel", reason)}
        >{summary}</ConfirmDialog>
      )}
      {kind === "retry" && (
        <ConfirmDialog title="Reintentar validación" reason="required" reasonRequiredMessage="Deja escrito por qué se reintenta." confirmLabel="Reintentar" confirmDisabled={!eligible.length} onClose={() => setKind(null)} onConfirm={(reason) => run("retry", reason)}>{summary}</ConfirmDialog>
      )}
      {outcome && (
        <FormModal title="Resultado" onClose={() => setOutcome(null)}>
          <div className="bulk-summary">
            <p><strong>{outcome.ok}</strong> {outcome.ok === 1 ? "orden" : "órdenes"} {SUCCESS_TEXT[outcome.kind]}.</p>
            {outcome.pendingConfirm > 0 && <p><strong>{outcome.pendingConfirm}</strong> en Incidencia «Cancelación por confirmar»: ya estaban en la ola de Invas.</p>}
            {outcome.failed.length > 0 && <IssueList title={`Fallaron ${outcome.failed.length}:`} issues={outcome.failed} />}
            {outcome.skipped.length > 0 && <IssueList title={`Se omitieron ${outcome.skipped.length}:`} issues={outcome.skipped} />}
          </div>
        </FormModal>
      )}
    </>
  );
}

function IssueList({ title, issues }: { title: string; issues: Issue[] }) {
  return (
    <div>
      <p>{title}</p>
      <ul className="bulk-issues">{issues.map((issue) => <li key={issue.ref}><strong>#{issue.ref}</strong>: {issue.reason}</li>)}</ul>
    </div>
  );
}
