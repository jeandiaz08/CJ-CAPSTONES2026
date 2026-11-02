"use client";

import type { ReactNode } from "react";
import { Modal } from "./Modal";

// Panel lateral derecho: se sigue viendo la tabla detrás.
export function SideDrawer({ title, onClose, footer, children }: {
  title: string;
  onClose: () => void;
  footer?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Modal title={title} onClose={onClose} panelClassName="side-drawer" backdropClassName="side-drawer-backdrop">
      <div className="form-modal-body">{children}</div>
      {footer && <div className="form-modal-foot">{footer}</div>}
    </Modal>
  );
}
