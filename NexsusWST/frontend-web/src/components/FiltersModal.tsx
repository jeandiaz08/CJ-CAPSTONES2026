"use client";

import type { ReactNode } from "react";
import { Modal } from "./Modal";

// Modal de filtros, centrado como los demás modales.
export function FiltersModal({ title, onClose, footer, children }: {
  title: string;
  onClose: () => void;
  footer?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Modal title={title} onClose={onClose} panelClassName="form-modal filters-modal">
      <div className="form-modal-body">{children}</div>
      {footer && <div className="form-modal-foot">{footer}</div>}
    </Modal>
  );
}
