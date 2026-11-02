import { useEffect, type ReactNode } from "react";

type AuthLayoutProps = {
  title: string;
  children: ReactNode;
  centered?: boolean;
  notification?: ReactNode;
};

function Brand() {
  return (
    <div className="auth-brand">
      <img
        className="auth-brand-icon"
        src={`${import.meta.env.BASE_URL}brand/logo-nexuswst.png`}
        alt=""
        aria-hidden="true"
      />
      <div>
        <div className="auth-brand-name">NexusWST</div>
        <div className="auth-brand-subtitle">WeStorage · Plataforma logística</div>
      </div>
    </div>
  );
}

export default function AuthLayout({ title, children, centered = false, notification }: AuthLayoutProps) {
  useEffect(() => {
    document.title = `${title} · NexusWST`;
    document.documentElement.lang = "es-CL";
  }, [title]);

  return (
    <div className="auth-shell">
      <aside className="auth-brand-panel" aria-label="NexusWST de WeStorage">
        <Brand />
        <div className="auth-marketing">
          <h2 className="auth-marketing-title">
            Órdenes, empaque y despacho <span className="auth-marketing-accent">en un solo lugar.</span>
          </h2>
          <p className="auth-marketing-copy">
            Enfócate en hacer crecer tus ventas, nosotros nos encargamos de tu logística.
          </p>
        </div>
        <footer className="auth-brand-footer">
          <img src={`${import.meta.env.BASE_URL}brand/logo-westorage.png`} alt="WeStorage" />
          <span>© WeStorage.cl</span>
        </footer>
      </aside>
      <main className="auth-content">
        <div className="auth-mobile-brand"><Brand /></div>
        <div className="auth-card-stack">
        {notification}
        <section
          className={`auth-card${centered ? " auth-card-centered" : ""}`}
          aria-labelledby="auth-title"
        >
          {children}
        </section>
        </div>
      </main>
    </div>
  );
}
