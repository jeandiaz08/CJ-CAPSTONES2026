import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';

// Tarea 07 · Fase 5: Clientes › Perfiles empresa (perfil Cliente).
const h = vi.hoisted(() => ({ session: null, calls: [], profiles: [], requests: [] }));
vi.mock('@/state/auth', () => ({ ROLE_LABELS: {}, useAuth: () => ({ session: h.session, companyId: null, logout: () => {} }) }));
vi.mock('@/components/Toast', () => ({ showToast: (m) => h.calls.push(['TOAST', m]) }));
vi.mock('@/services/api', () => {
  class ApiError extends Error { constructor(m, status, code, fields = []) { super(m); this.status = status; this.code = code; this.fields = fields; } }
  return {
    ApiError, apiErrorMessage: (e) => e.message, withCompany: (p) => p,
    downloadFile: async (path) => { h.calls.push(['DOWNLOAD', path]); },
    api: async (service, path, init) => {
      const body = init?.body ? JSON.parse(init.body) : undefined;
      h.calls.push([init?.method ?? 'GET', path, body]);
      if (path === '/v1/clients') return { data: [{ id: 1, name: 'Empresa Demo Uno SpA', active: true }] };
      if (!init?.method && path.startsWith('/v1/profiles?')) return { data: h.profiles, total: h.profiles.length, limit: 20, offset: 0 };
      if (!init?.method && path.startsWith('/v1/profile-requests')) {
        const status = new URLSearchParams(path.split('?')[1]).get('status');
        const data = status ? h.requests.filter((r) => r.status === status) : h.requests;
        return { data, total: data.length, counts: { pending: 1, approved: 0, rejected: 1 }, limit: 20, offset: 0 };
      }
      if (init?.method === 'POST' && path === '/v1/profile-requests') return { id: 99, status: 'pending', ...body };
      return { data: [] };
    },
  };
});

const profile = (id, extra = {}) => ({ id, name: 'Perfil ' + id, email: 'p' + id + '@demouno.cl', hasEmail: true, phone: '', jobTitle: '', role: 'cliente', permissions: [], companyId: 1, companyName: 'Empresa Demo Uno SpA', active: true, linked: false, createdAt: '2026-10-14T13:00:00Z', lastLoginAt: null, ...extra });
const request = (id, extra = {}) => ({ id, companyId: 1, companyName: 'Empresa Demo Uno SpA', kind: 'deactivation', kindLabel: 'Desactivación', targetUserId: 2,
  profile: { name: 'Bruno Salinas', email: 'bruno@demouno.cl', phone: '', jobTitle: '' }, reason: 'Ya no trabaja en la empresa', status: 'pending',
  requestedBy: { id: 5, name: 'Cliente Demo Uno' }, requestedAt: '2026-11-08T12:00:00Z', reviewedBy: null, reviewedAt: null, rejectionReason: '', createdUserId: null, ...extra });
const settle = async () => { for (let i = 0; i < 8; i++) await act(async () => { await new Promise((r) => setTimeout(r, 15)); }); };
const click = async (e) => { expect(e, 'no encontrado').toBeTruthy(); await act(async () => e.click()); await settle(); };
const setValue = async (el, v) => {
  const proto = el.tagName === 'TEXTAREA' ? HTMLTextAreaElement : HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(proto.prototype, 'value').set.call(el, v); el.dispatchEvent(new Event('input', { bubbles: true })); });
};
const button = (root, text) => [...root.querySelectorAll('button, a')].find((b) => b.textContent.trim() === text);

describe('Clientes › Perfiles empresa', () => {
  let root, host;
  const mount = async (search) => {
    globalThis.__nav = { params: new URLSearchParams(search), calls: [], pathname: '/clientes/perfiles-empresa' };
    const { CompanyProfilesView } = await import('@/features/clients');
    await act(async () => root.render(<CompanyProfilesView />));
    await settle();
  };
  beforeEach(() => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
    globalThis.matchMedia = () => ({ matches: false, addEventListener() {}, removeEventListener() {} });
    h.calls.length = 0;
    h.profiles = [profile(2, { name: 'Bruno Salinas' }), profile(3, { name: 'Pablo Ruiz', active: false })];
    h.requests = [request(10), request(11, { kind: 'new_profile', kindLabel: 'Nuevo perfil', targetUserId: null, profile: { name: 'Jorge Vidal', email: 'jorge@demouno.cl', phone: '', jobTitle: '' }, reason: 'Apoyo temporal', status: 'rejected', reviewedBy: { id: 1, name: 'Admin Demo' }, reviewedAt: '2026-11-08T14:00:00Z', rejectionReason: 'Ya existe un perfil' })];
    h.session = { user: { id: 5, role: 'cliente', permissions: [], name: 'Cliente Demo Uno', allCompanies: false, companies: [1] } };
    host = document.createElement('div'); document.body.append(host); root = createRoot(host);
  });
  afterEach(async () => { await act(async () => root.unmount()); host.remove(); document.body.innerHTML = ''; });

  it('Mis perfiles: título con la empresa, columnas y «Solicitud pendiente» en la fila', async () => {
    await mount('');
    expect([...host.querySelectorAll('.view-tabs a')].map((a) => a.textContent)).toEqual(['Mis perfiles', 'Mis solicitudes']);
    expect(host.querySelector('.section-title').textContent).toBe('Perfiles Empresa Demo Uno SpA');
    expect([...host.querySelectorAll('.clients-list th')].map((t) => t.textContent)).toEqual(['', 'Nombre completo', 'Correo', 'Teléfono', 'Estado', 'Fecha de creación']);
    const [bruno, pablo] = [...host.querySelectorAll('.clients-list tbody tr')];
    expect(bruno.querySelector('.status-cell').textContent).toBe('ActivoSolicitud pendiente');
    expect(pablo.querySelector('.status-cell').textContent).toBe('Inactivo');
    for (const t of ['Exportar Excel', 'Solicitar nuevo perfil']) expect(button(host, t)).toBeTruthy();
    expect(button(host, 'Nuevo perfil')).toBeUndefined();
  });

  it('panel: el botón que corresponde, deshabilitado si ya hay una pendiente; pide motivo', async () => {
    await mount('perfil=2');
    const pending = button(host.querySelector('.clients-panel'), 'Solicitud pendiente');
    expect(pending.disabled).toBe(true);
    expect(button(host.querySelector('.clients-panel'), 'Solicitar desactivación')).toBeUndefined();
    await act(async () => root.unmount()); root = createRoot(host);
    await mount('perfil=3');
    const panel = host.querySelector('.clients-panel');
    expect(button(panel, 'Solicitar desactivación')).toBeUndefined();
    await click(button(panel, 'Solicitar activación'));
    const modal = document.querySelector('.form-modal');
    await click(button(modal, 'Enviar solicitud'));
    expect(modal.textContent).toContain('Indica el motivo de la solicitud.');
    await setValue(modal.querySelector('textarea'), 'Vuelve de vacaciones');
    await click(button(modal, 'Enviar solicitud'));
    expect(h.calls.find(([m, p]) => m === 'POST' && p === '/v1/profile-requests')[2]).toEqual({ kind: 'activation', targetUserId: 3, reason: 'Vuelve de vacaciones' });
    expect(h.calls.some(([k, m]) => k === 'TOAST' && m === 'Solicitud enviada; te avisaremos cuando el Administrador la revise.')).toBe(true);
  });

  it('Solicitar nuevo perfil: nombre, correo y motivo obligatorios', async () => {
    await mount('');
    await click(button(host, 'Solicitar nuevo perfil'));
    const modal = document.querySelector('.form-modal');
    await click(button(modal, 'Enviar solicitud'));
    expect([...modal.querySelectorAll('.modal-field-error')].map((e) => e.textContent)).toEqual([
      'El nombre es obligatorio.', 'El correo es obligatorio: el perfil recibe ahí el enlace para definir su contraseña.', 'Indica el motivo u observación (al menos 5 caracteres).']);
    const [name, , email] = modal.querySelectorAll('input');
    await setValue(name, 'Luis Pérez');
    await setValue(email, 'luis@demouno.cl');
    await setValue(modal.querySelector('textarea'), 'Nuevo encargado en bodega');
    await click(button(modal, 'Enviar solicitud'));
    expect(h.calls.find(([m, p]) => m === 'POST' && p === '/v1/profile-requests')[2]).toEqual({ kind: 'new_profile', name: 'Luis Pérez', email: 'luis@demouno.cl', phone: '', jobTitle: '', reason: 'Nuevo encargado en bodega' });
  });

  it('Mis solicitudes: KPIs, columnas del Cliente, estado de la solicitud y sin acciones', async () => {
    await mount('tab=solicitudes&solicitud=11');
    expect([...host.querySelectorAll('.kpi-label')].map((k) => k.textContent)).toEqual(['Pendientes', 'Aprobadas', 'Rechazadas']);
    expect([...host.querySelectorAll('.clients-list th')].map((t) => t.textContent)).toEqual(['Motivo', 'Tipo', 'Nombre perfil', 'Estado', 'Fecha de solicitud']);
    expect(host.querySelector('.orders-search')).toBeNull();
    const panel = host.querySelector('.clients-panel');
    expect(panel.querySelector('.badge').textContent).toBe('Rechazada');
    expect(panel.textContent).toContain('Ya existe un perfil');
    for (const t of ['Aceptar solicitud', 'Rechazar']) expect(button(panel, t)).toBeUndefined();
  });
});
