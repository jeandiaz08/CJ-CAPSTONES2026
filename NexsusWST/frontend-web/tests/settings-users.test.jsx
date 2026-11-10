import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import { canAccessPath, visibleMenu } from '@/state/menuAccess';

// Tarea 08 · Fase 1: Configuración › Usuarios WeStorage (perfiles con scope=internal).
const h = vi.hoisted(() => ({ session: null, calls: [], profiles: [], created: null, reset: null }));
vi.mock('@/state/auth', () => ({ ROLE_LABELS: {}, useAuth: () => ({ session: h.session, companyId: null, logout: () => {} }) }));
vi.mock('@/services/api', () => {
  class ApiError extends Error { constructor(m, status, code, fields = []) { super(m); this.status = status; this.code = code; this.fields = fields; } }
  return {
    ApiError, apiErrorMessage: (e) => e.message, withCompany: (p) => p, downloadFile: async () => {},
    api: async (service, path, init) => {
      const body = init?.body instanceof FormData ? 'form' : init?.body ? JSON.parse(init.body) : undefined;
      h.calls.push([init?.method ?? 'GET', path, body]);
      if (!init?.method && path.startsWith('/v1/profiles?')) return { data: h.profiles, total: h.profiles.length, limit: 20, offset: 0 };
      if (init?.method === 'POST' && path.startsWith('/v1/profiles?scope')) return h.created(body);
      if (/^\/v1\/profiles\/\d+\/password/.test(path)) return h.reset;
      return { data: [] };
    },
  };
});

const profile = (id, extra = {}) => ({ id, name: 'Perfil ' + id, email: 'p' + id + '@westorage.cl', hasEmail: true, phone: '', jobTitle: '', role: 'empaque', permissions: ['despacho'], companyId: 0, companyName: '', active: true, linked: true, createdAt: '2026-10-14T13:00:00Z', lastLoginAt: null, ...extra });
const user = (role, permissions = [], id = 1) => ({ user: { id, role, permissions, name: 'X', allCompanies: true, companies: [] } });
const settle = async () => { for (let i = 0; i < 8; i++) await act(async () => { await new Promise((r) => setTimeout(r, 15)); }); };
const click = async (e) => { expect(e, 'no encontrado').toBeTruthy(); await act(async () => e.click()); await settle(); };
const setValue = async (el, v) => {
  const proto = el.tagName === 'SELECT' ? HTMLSelectElement : HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(proto.prototype, 'value').set.call(el, v); el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input', { bubbles: true })); });
};
const button = (root, text) => [...root.querySelectorAll('button, a')].find((b) => b.textContent.trim() === text);
const checkbox = (root, text) => [...root.querySelectorAll('label.checkbox-row')].find((l) => l.textContent.trim() === text)?.querySelector('input');

describe('Menú: Configuración › Usuarios WeStorage', () => {
  it('solo el Administrador; «Ajustes generales» es Próximamente', () => {
    const conf = visibleMenu({ role: 'admin', permissions: [] }).find((e) => e.name === 'Configuración');
    expect(conf.disabled).toBe(false);
    expect(conf.children.map((c) => [c.name, c.disabled])).toEqual([['Usuarios WeStorage', false], ['Ajustes generales', true]]);
    for (const role of ['jefe_operaciones', 'empaque', 'cliente']) {
      expect(visibleMenu({ role, permissions: ['gestionar_perfiles'] }).some((e) => e.name === 'Configuración')).toBe(false);
      expect(canAccessPath({ role, permissions: ['gestionar_perfiles'] }, '/configuracion/usuarios')).toBe(false);
    }
    expect(canAccessPath({ role: 'admin', permissions: [] }, '/configuracion/usuarios')).toBe(true);
    expect(canAccessPath({ role: 'admin', permissions: [] }, '/configuracion/ajustes')).toBe(false);
  });
});

describe('Usuarios WeStorage › Perfiles', () => {
  let root, host;
  const mount = async (search) => {
    globalThis.__nav = { params: new URLSearchParams(search), calls: [], pathname: '/configuracion/usuarios' };
    const { InternalUsersView } = await import('@/features/settings');
    await act(async () => root.render(<InternalUsersView />));
    await settle();
  };
  beforeEach(() => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
    globalThis.matchMedia = () => ({ matches: false, addEventListener() {}, removeEventListener() {} });
    h.calls.length = 0;
    h.profiles = [
      profile(2, { name: 'María Rojas', role: 'jefe_operaciones' }),
      profile(3, { name: 'Juan Bodega', email: '', username: 'juan.bodega', hasEmail: false }),
      profile(1, { name: 'Ana Admin', role: 'admin', permissions: ['gestionar_perfiles'] }),
    ];
    h.session = user('admin', ['gestionar_perfiles']);
    host = document.createElement('div'); document.body.append(host); root = createRoot(host);
  });
  afterEach(async () => { await act(async () => root.unmount()); host.remove(); document.body.innerHTML = ''; });

  it('«Perfiles WeStorage»: columnas, usuario sin correo, rol y permisos en chips; alcance internal', async () => {
    await mount('');
    expect(host.querySelector('.section-title').textContent).toBe('Perfiles WeStorage');
    expect([...host.querySelectorAll('.clients-list th')].map((t) => t.textContent)).toEqual(['', 'Nombre completo', 'Correo o usuario', 'Rol', 'Permisos', 'Estado', 'Fecha de creación']);
    const rows = [...host.querySelectorAll('.clients-list tbody tr')].map((r) => [...r.querySelectorAll('td')].slice(1, 5).map((c) => c.textContent));
    expect(rows).toEqual([
      ['María Rojas', 'p2@westorage.cl', 'Jefe de Operaciones', 'Despacho'],
      ['Juan Bodega', 'Usuario: juan.bodega', 'Empaque', 'Despacho'],
      ['Ana Admin', 'p1@westorage.cl', 'Administrador', 'Gestionar perfiles'],
    ]);
    expect(host.querySelectorAll('.perm-chips .badge').length).toBe(3);
    expect(h.calls.some(([, p]) => p.startsWith('/v1/profiles?') && p.includes('scope=internal'))).toBe(true);
    expect(h.calls.some(([, p]) => p === '/v1/clients')).toBe(false); // sin empresa
    expect(host.querySelector('.orders-search input').placeholder).toBe('Buscar nombre o usuario');
  });

  it('panel sin empresa; sin «Gestionar perfiles» queda en lectura', async () => {
    await mount('perfil=3');
    const facts = Object.fromEntries([...host.querySelectorAll('.clients-panel dl > div')].map((d) => [d.querySelector('dt').textContent, d.querySelector('dd').textContent]));
    expect(facts).toMatchObject({ Usuario: 'juan.bodega', Rol: 'Empaque', Permisos: 'Despacho' });
    expect(facts.Empresa).toBeUndefined();
    expect(button(host.querySelector('.clients-panel'), 'Restablecer contraseña').disabled).toBe(false); // sin correo: temporal
    await act(async () => root.unmount()); root = createRoot(host);
    h.session = user('admin');
    await mount('perfil=3');
    expect(button(host, 'Nuevo perfil')).toBeUndefined();
    for (const t of ['Editar', 'Desactivar', 'Restablecer contraseña']) expect(button(host.querySelector('.clients-panel'), t)).toBeUndefined();
  });

  it('Nuevo perfil: rol obligatorio, permisos según el rol y usuario sin correo con contraseña temporal', async () => {
    h.created = (body) => profile(20, { name: body.name, email: '', username: body.username, hasEmail: false, role: body.role, permissions: body.permissions, temporaryPassword: 'Tmp-12345678' });
    await mount('');
    await click(button(host, 'Nuevo perfil'));
    const modal = document.querySelector('.form-modal');
    expect(modal.textContent).not.toContain('Empresa');
    await click(button(modal, 'Crear perfil'));
    expect([...modal.querySelectorAll('.modal-field-error')].map((e) => e.textContent)).toEqual(['El nombre es obligatorio.', 'Indica el correo o marca «Usuario sin correo».', 'Elige el rol.']);
    const role = modal.querySelector('select');
    expect([...role.options].map((o) => o.textContent)).toEqual(['Elige el rol', 'Administrador', 'Jefe de Operaciones', 'Empaque']);
    await setValue(role, 'admin');
    expect(checkbox(modal, 'Gestionar perfiles')).toBeTruthy();
    expect(checkbox(modal, 'Despacho')).toBeUndefined();
    await click(checkbox(modal, 'Gestionar perfiles'));
    await setValue(role, 'empaque'); // el permiso del Administrador se quita
    await click(checkbox(modal, 'Despacho'));
    await setValue(modal.querySelector('input'), 'Pedro Bodega');
    await click(checkbox(modal, 'Usuario sin correo'));
    await setValue(modal.querySelector('input[autocomplete=off]'), 'Pedro.Bodega');
    await click(button(modal, 'Crear perfil'));
    const post = h.calls.find(([m, p]) => m === 'POST' && p.startsWith('/v1/profiles?scope=internal'));
    expect(post[2]).toEqual({ name: 'Pedro Bodega', phone: '', jobTitle: '', role: 'empaque', permissions: ['despacho'], username: 'pedro.bodega' });
    const dialog = [...document.querySelectorAll('.form-modal')].find((m) => m.textContent.includes('Contraseña temporal'));
    expect(dialog.querySelector('code').textContent).toBe('Tmp-12345678');
  });

  it('Editar el propio perfil: rol y permisos bloqueados', async () => {
    await mount('perfil=1');
    await click(button(host.querySelector('.clients-panel'), 'Editar'));
    const modal = document.querySelector('.form-modal');
    expect(modal.querySelector('select').disabled).toBe(true);
    expect(checkbox(modal, 'Gestionar perfiles').disabled).toBe(true);
    expect(modal.textContent).toContain('No puedes cambiar tu propio rol ni tus permisos.');
  });

  it('Restablecer sin correo: contraseña temporal una sola vez', async () => {
    h.reset = { emailSent: false, temporaryPassword: 'Tmp-ABCDEFGH' };
    await mount('perfil=3');
    await click(button(host.querySelector('.clients-panel'), 'Restablecer contraseña'));
    const confirm = [...document.querySelectorAll('[role=dialog]')].at(-1);
    expect(confirm.textContent).toContain('contraseña temporal');
    await click(button(confirm, 'Restablecer'));
    expect(document.body.textContent).toContain('Tmp-ABCDEFGH');
  });
});
