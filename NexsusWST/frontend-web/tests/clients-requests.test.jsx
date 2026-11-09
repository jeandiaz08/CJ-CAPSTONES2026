import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';

// Tarea 07 · Fase 4: Administración clientes › Solicitudes.
const h = vi.hoisted(() => ({ session: null, calls: [], requests: [] }));
vi.mock('@/state/auth', () => ({ ROLE_LABELS: {}, useAuth: () => ({ session: h.session, companyId: null, logout: () => {} }) }));
vi.mock('@/services/api', () => {
  class ApiError extends Error { constructor(m, status, code, fields = []) { super(m); this.status = status; this.code = code; this.fields = fields; } }
  return {
    ApiError, apiErrorMessage: (e) => e.message, withCompany: (p) => p, downloadFile: async () => {},
    api: async (service, path, init) => {
      const body = init?.body ? JSON.parse(init.body) : undefined;
      h.calls.push([init?.method ?? 'GET', path, body]);
      if (!init?.method && path.startsWith('/v1/profile-requests')) return { data: h.requests, total: h.requests.length, counts: { pending: 3, approved: 1, rejected: 1 }, limit: 20, offset: 0 };
      const m = path.match(/^\/v1\/profile-requests\/(\d+)\/(approve|reject)/);
      if (m) return { ...h.requests.find((r) => r.id === Number(m[1])), status: m[2] === 'approve' ? 'approved' : 'rejected' };
      return { data: [] };
    },
  };
});

const request = (id, extra = {}) => ({ id, companyId: 1, companyName: 'Andes', kind: 'new_profile', kindLabel: 'Nuevo perfil', targetUserId: null,
  profile: { name: 'Luis Pérez', email: 'luis@andes.cl', phone: '', jobTitle: 'Bodega' }, reason: 'Nuevo encargado en bodega', status: 'pending',
  requestedBy: { id: 5, name: 'Esperanza Gómez' }, requestedAt: '2026-11-08T12:00:00Z', reviewedBy: null, reviewedAt: null, rejectionReason: '', createdUserId: null, ...extra });
const user = (role, permissions = []) => ({ user: { id: 1, role, permissions, name: 'X', allCompanies: true, companies: [] } });
const settle = async () => { for (let i = 0; i < 8; i++) await act(async () => { await new Promise((r) => setTimeout(r, 15)); }); };
const click = async (e) => { expect(e, 'no encontrado').toBeTruthy(); await act(async () => e.click()); await settle(); };
const setValue = async (el, v) => {
  const proto = el.tagName === 'TEXTAREA' ? HTMLTextAreaElement : HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(proto.prototype, 'value').set.call(el, v); el.dispatchEvent(new Event('input', { bubbles: true })); });
};
const button = (root, text) => [...root.querySelectorAll('button, a')].find((b) => b.textContent.trim() === text);

describe('Administración clientes › Solicitudes', () => {
  let root, host;
  const mount = async (search) => {
    globalThis.__nav = { params: new URLSearchParams(search), calls: [], pathname: '/clientes/administracion' };
    const { ClientAdminView } = await import('@/features/clients');
    await act(async () => root.render(<ClientAdminView />));
    await settle();
  };
  beforeEach(() => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
    globalThis.matchMedia = () => ({ matches: false, addEventListener() {}, removeEventListener() {} });
    h.calls.length = 0;
    h.requests = [request(1), request(2, { kind: 'deactivation', kindLabel: 'Desactivación', targetUserId: 9, profile: { name: 'Bruno Salinas', email: 'bruno@andes.cl', phone: '', jobTitle: '' }, reason: 'Ya no trabaja' }),
      request(3, { status: 'rejected', reviewedBy: { id: 1, name: 'Admin Demo' }, reviewedAt: '2026-11-08T14:00:00Z', rejectionReason: 'Ya existe un perfil' })];
    h.session = user('admin', ['gestionar_perfiles']);
    host = document.createElement('div'); document.body.append(host); root = createRoot(host);
  });
  afterEach(async () => { await act(async () => root.unmount()); host.remove(); document.body.innerHTML = ''; });

  it('KPIs que filtran, columnas y búsqueda por empresa', async () => {
    await mount('tab=solicitudes');
    expect([...host.querySelectorAll('.kpi-label')].map((k) => k.textContent)).toEqual(['Pendientes', 'Aprobadas', 'Rechazadas']);
    expect([...host.querySelectorAll('.clients-list th')].map((t) => t.textContent)).toEqual(['Empresa', 'Tipo', 'Solicitado por', 'Nombre perfil', 'Estado', 'Fecha de solicitud']);
    expect(host.querySelector('.orders-search input').placeholder).toBe('Buscar nombre de empresa');
    await click(host.querySelector('.kpi-button'));
    expect(globalThis.__nav.calls.at(-1)[1]).toContain('estado=pending');
  });

  it('panel: sin rol ni permisos; respondida muestra quién, cuándo y el motivo del rechazo', async () => {
    await mount('tab=solicitudes&solicitud=3');
    const panel = host.querySelector('.clients-panel');
    expect(panel.querySelector('h2').textContent).toBe('Detalle de solicitud');
    const facts = Object.fromEntries([...panel.querySelectorAll('dl > div')].map((d) => [d.querySelector('dt').textContent, d.querySelector('dd').textContent]));
    expect(facts).toMatchObject({ 'Nombre completo': 'Luis Pérez', Correo: 'luis@andes.cl', Cargo: 'Bodega', Tipo: 'Nuevo perfil', 'Revisado por': 'Admin Demo', 'Motivo del rechazo': 'Ya existe un perfil' });
    expect(facts.Rol).toBeUndefined();
    expect(facts.Permisos).toBeUndefined();
    expect(button(panel, 'Aceptar solicitud')).toBeUndefined(); // ya respondida
  });

  it('pendiente de perfil nuevo: Aceptar, Editar y aceptar y Rechazar (con motivo obligatorio)', async () => {
    await mount('tab=solicitudes&solicitud=1');
    const panel = host.querySelector('.clients-panel');
    for (const t of ['Aceptar solicitud', 'Editar y aceptar', 'Rechazar']) expect(button(panel, t)).toBeTruthy();
    await click(button(panel, 'Rechazar'));
    const modal = document.querySelector('.form-modal');
    await click(button(modal, 'Rechazar'));
    expect(modal.textContent).toContain('Indica el motivo del rechazo.');
    await setValue(modal.querySelector('textarea'), 'Ya existe un perfil');
    await click(button(modal, 'Rechazar'));
    expect(h.calls.find(([m, p]) => m === 'POST' && p === '/v1/profile-requests/1/reject')[2]).toEqual({ reason: 'Ya existe un perfil' });
  });

  it('Editar y aceptar: formulario con los datos, empresa fija y aprueba con lo editado', async () => {
    await mount('tab=solicitudes&solicitud=1');
    await click(button(host.querySelector('.clients-panel'), 'Editar y aceptar'));
    const modal = document.querySelector('.form-modal');
    expect(modal.querySelector('h2, .modal-title')?.textContent ?? modal.textContent).toContain('Editar y aceptar solicitud');
    expect(modal.querySelector('select').disabled).toBe(true);
    const [name] = modal.querySelectorAll('input');
    expect(name.value).toBe('Luis Pérez');
    await setValue(name, 'Luis A. Pérez');
    await click(button(modal, 'Aceptar solicitud'));
    expect(h.calls.find(([m, p]) => m === 'POST' && p === '/v1/profile-requests/1/approve')[2]).toEqual({ name: 'Luis A. Pérez', email: 'luis@andes.cl', phone: '', jobTitle: 'Bodega' });
  });

  it('activación o desactivación: sin «Editar y aceptar»; sin el permiso, sin acciones', async () => {
    await mount('tab=solicitudes&solicitud=2');
    expect(button(host.querySelector('.clients-panel'), 'Editar y aceptar')).toBeUndefined();
    expect(button(host.querySelector('.clients-panel'), 'Aceptar solicitud')).toBeTruthy();
    await act(async () => root.unmount()); root = createRoot(host);
    h.session = user('admin');
    await mount('tab=solicitudes&solicitud=1');
    for (const t of ['Aceptar solicitud', 'Editar y aceptar', 'Rechazar']) expect(button(host.querySelector('.clients-panel'), t)).toBeUndefined();
  });
});
