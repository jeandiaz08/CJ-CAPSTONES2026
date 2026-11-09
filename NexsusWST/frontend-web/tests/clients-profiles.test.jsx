import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';

// Tarea 07 · Fase 3: Administración clientes › Perfiles.
const h = vi.hoisted(() => ({ session: null, calls: [], downloads: [], profiles: [], importResult: null }));
vi.mock('@/state/auth', () => ({ ROLE_LABELS: {}, useAuth: () => ({ session: h.session, companyId: null, logout: () => {} }) }));
vi.mock('@/services/api', () => {
  class ApiError extends Error { constructor(m, status, code, fields = []) { super(m); this.status = status; this.code = code; this.fields = fields; } }
  return {
    ApiError, apiErrorMessage: (e) => e.message, withCompany: (p) => p,
    downloadFile: async (path, name, service) => { h.downloads.push([path, name, service]); },
    api: async (service, path, init) => {
      const body = init?.body instanceof FormData ? 'form' : init?.body ? JSON.parse(init.body) : undefined;
      h.calls.push([init?.method ?? 'GET', path, body]);
      if (path === '/v1/clients') return { data: [{ id: 1, name: 'Andes', active: true }, { id: 2, name: 'Montaña Sur', active: false }] };
      if (!init?.method && path.startsWith('/v1/profiles?')) return { data: h.profiles, total: h.profiles.length, limit: 20, offset: 0 };
      if (path.startsWith('/v1/profiles/import')) return h.importResult;
      if (init?.method === 'POST' && path.startsWith('/v1/profiles?scope')) {
        return { ...profile(20), name: body.name, email: body.email, hasEmail: true };
      }
      const pw = path.match(/^\/v1\/profiles\/(\d+)\/password/);
      if (pw) return { emailSent: true };
      return { data: [] };
    },
  };
});

const profile = (id, extra = {}) => ({ id, name: 'Perfil ' + id, email: 'p' + id + '@andes.cl', hasEmail: true, phone: '+56 9 1111 2222', jobTitle: 'Bodega', role: 'cliente', permissions: [], companyId: 1, companyName: 'Andes', active: true, linked: true, createdAt: '2026-10-14T13:00:00Z', lastLoginAt: null, ...extra });
const user = (role, permissions = []) => ({ user: { id: 1, role, permissions, name: 'X', allCompanies: true, companies: [] } });
const settle = async () => { for (let i = 0; i < 8; i++) await act(async () => { await new Promise((r) => setTimeout(r, 15)); }); };
const click = async (e) => { expect(e, 'no encontrado').toBeTruthy(); await act(async () => e.click()); await settle(); };
const setValue = async (el, v) => {
  const proto = el.tagName === 'SELECT' ? HTMLSelectElement : el.tagName === 'TEXTAREA' ? HTMLTextAreaElement : HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(proto.prototype, 'value').set.call(el, v); el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input', { bubbles: true })); });
};
const button = (root, text) => [...root.querySelectorAll('button, a')].find((b) => b.textContent.trim() === text);

describe('Administración clientes › Perfiles', () => {
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
    h.calls.length = 0; h.downloads.length = 0;
    h.profiles = [profile(5, { name: 'Esperanza Gómez', lastLoginAt: '2026-11-01T12:00:00Z' }), profile(6, { name: 'Pablo Carpio', email: 'pablo.carpio@andes.cl', active: false })];
    h.session = user('admin', ['gestionar_perfiles']);
    host = document.createElement('div'); document.body.append(host); root = createRoot(host);
  });
  afterEach(async () => { await act(async () => root.unmount()); host.remove(); document.body.innerHTML = ''; });

  it('pestaña por defecto: «Todos los perfiles», columnas y alcance client', async () => {
    await mount('');
    expect(host.querySelector('.section-title').textContent).toBe('Todos los perfiles');
    expect([...host.querySelectorAll('.clients-list th')].map((t) => t.textContent)).toEqual(['', 'Nombre perfil', 'Nombre empresa', 'Correo', 'Teléfono', 'Estado', 'Fecha de creación']);
    const rows = [...host.querySelectorAll('.clients-list tbody tr')].map((r) => [...r.querySelectorAll('td')].slice(1, 6).map((c) => c.textContent));
    expect(rows).toEqual([['Esperanza Gómez', 'Andes', 'p5@andes.cl', '+56 9 1111 2222', 'Activo'], ['Pablo Carpio', 'Andes', 'pablo.carpio@andes.cl', '+56 9 1111 2222', 'Inactivo']]);
    expect(h.calls.some(([, p]) => p.startsWith('/v1/profiles?') && p.includes('scope=client'))).toBe(true);
    expect(host.querySelector('.orders-search input').placeholder).toBe('Buscar nombre del perfil o empresa');
  });

  it('panel: rol Cliente sin permisos, último acceso si existe y solo el botón que corresponde', async () => {
    await mount('tab=perfiles&perfil=5');
    const panel = host.querySelector('.clients-panel');
    expect(panel.querySelector('h2').textContent).toBe('Detalle de perfil Esperanza Gómez');
    const facts = Object.fromEntries([...panel.querySelectorAll('dl > div')].map((d) => [d.querySelector('dt').textContent, d.querySelector('dd').textContent]));
    expect(facts).toMatchObject({ Rol: 'Cliente', Permisos: 'Ninguno', Empresa: 'Andes', Cargo: 'Bodega' });
    expect(facts['Último acceso']).toBeTruthy();
    expect(button(panel, 'Desactivar')).toBeTruthy();
    expect(button(panel, 'Activar')).toBeUndefined();
    expect(button(panel, 'Restablecer contraseña')).toBeTruthy();
    await act(async () => root.unmount()); root = createRoot(host);
    await mount('tab=perfiles&perfil=6');
    const p2 = host.querySelector('.clients-panel');
    expect(p2.textContent).not.toContain('Último acceso');
    expect(button(p2, 'Activar')).toBeTruthy();
    expect(button(p2, 'Desactivar')).toBeUndefined();
  });

  it('sin «Gestionar perfiles»: sin Nuevo perfil, botones ni carga masiva', async () => {
    h.session = user('admin');
    await mount('tab=perfiles&perfil=5');
    expect(button(host, 'Nuevo perfil')).toBeUndefined();
    for (const t of ['Editar', 'Desactivar', 'Activar', 'Restablecer contraseña']) expect(button(host.querySelector('.clients-panel'), t)).toBeUndefined();
    await click(button(host, 'Excel acciones'));
    const items = [...host.querySelectorAll('[role=menuitem]')].map((b) => b.textContent);
    expect(items.some((t) => t.includes('Importar') || t.includes('plantilla'))).toBe(false);
    expect(items.some((t) => t.startsWith('Exportar'))).toBe(true);
  });

  it('Nuevo perfil: correo obligatorio, sin «Usuario sin correo» y rol Cliente fijo', async () => {
    await mount('tab=perfiles');
    await click(button(host, 'Nuevo perfil'));
    const modal = document.querySelector('.form-modal');
    expect(modal.textContent).toContain('Rol: Cliente');
    expect(modal.textContent).not.toContain('Usuario sin correo');
    expect(modal.querySelector('input[type=checkbox]')).toBeNull();
    await click(button(modal, 'Crear perfil'));
    expect([...modal.querySelectorAll('.modal-field-error')].map((e) => e.textContent)).toEqual(['El nombre es obligatorio.', 'Elige la empresa.', 'El correo es obligatorio: el perfil recibe ahí el enlace para definir su contraseña.']);
    const [name] = modal.querySelectorAll('input');
    await setValue(name, 'Luis Nuevo');
    const select = modal.querySelector('select');
    expect([...select.options].map((o) => o.textContent)).toEqual(['Elige la empresa', 'Andes']); // solo empresas activas
    await setValue(select, '1');
    await setValue(modal.querySelector('input[type=email]'), 'luis@andes.cl');
    await click(button(modal, 'Crear perfil'));
    expect(h.calls.find(([m, p]) => m === 'POST' && p === '/v1/profiles?scope=client')[2]).toEqual({ name: 'Luis Nuevo', companyId: 1, phone: '', jobTitle: '', email: 'luis@andes.cl' });
    expect(document.querySelector('.form-modal')).toBeNull();
  });

  it('Restablecer contraseña: siempre por enlace al correo', async () => {
    await mount('tab=perfiles&perfil=5');
    await click(button(host.querySelector('.clients-panel'), 'Restablecer contraseña'));
    const modal = document.querySelector('.form-modal');
    expect(modal.textContent).toContain('Le enviaremos a p5@andes.cl un enlace');
    await click(button(modal, 'Restablecer'));
    expect(h.calls.some(([m, p]) => m === 'POST' && p === '/v1/profiles/5/password?scope=client')).toBe(true);
    expect(document.querySelector('.form-modal')).toBeNull();
  });

  it('Importar perfiles: informa las filas inválidas con su fila y motivo', async () => {
    h.importResult = { rows: 3, created: [{ row: 2, id: 30, name: 'Ana', login: 'ana@x.cl' }], errors: [{ row: 3, message: 'Falta el RUT de la empresa.' }, { row: 4, message: 'Correo inválido.' }] };
    await mount('tab=perfiles');
    await click(button(host, 'Excel acciones'));
    await click(button(host, 'Importar perfiles'));
    const modal = document.querySelector('.form-modal');
    const input = modal.querySelector('input[type=file]');
    await act(async () => {
      Object.defineProperty(input, 'files', { value: [new File(['x'], 'perfiles.xlsx')], configurable: true });
      input.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await click(button(modal, 'Importar'));
    const result = document.querySelector('.form-modal');
    expect(result.textContent).toContain('1 perfil creado');
    expect([...result.querySelectorAll('tbody tr')].map((r) => r.textContent)).toEqual(['3Falta el RUT de la empresa.', '4Correo inválido.']);
  });
});
