import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import { normalizeRut } from '@/features/clients/rut';
import { waveOptions } from '@/features/clients/waves';

// Tarea 07 · Fase 2: Administración clientes › Empresas.
const h = vi.hoisted(() => ({ session: null, calls: [], downloads: [], companies: [], failCreate: null }));
vi.mock('@/state/auth', () => ({ ROLE_LABELS: {}, useAuth: () => ({ session: h.session, companyId: null, logout: () => {} }) }));
vi.mock('@/services/api', () => {
  class ApiError extends Error { constructor(m, status, code, fields = []) { super(m); this.status = status; this.code = code; this.fields = fields; } }
  return {
    ApiError, apiErrorMessage: (e) => e.message, withCompany: (p) => p,
    downloadFile: async (path, name, service) => { h.downloads.push([path, name, service]); },
    api: async (service, path, init) => {
      h.calls.push([init?.method ?? 'GET', path, init?.body ? JSON.parse(init.body) : undefined]);
      if (path === '/v1/waves') return { data: [{ id: 'OLA-1', code: 'AM', name: 'Ola mañana', type: 'automatica' }] };
      if (path.startsWith('/v1/companies?')) return { data: h.companies, total: h.companies.length, limit: 20, offset: 0 };
      if (init?.method === 'POST' && path === '/v1/companies') {
        if (h.failCreate) throw h.failCreate;
        return { ...company(9), ...JSON.parse(init.body) };
      }
      const m = path.match(/^\/v1\/companies\/(\d+)(\/(disable|enable))?$/);
      if (m) return { ...h.companies.find((c) => c.id === Number(m[1])), active: m[3] === 'enable' };
      return { data: [] };
    },
  };
});

const company = (id, extra = {}) => ({ id, name: 'Empresa ' + id, taxId: '76.123.456-0', email: 'e' + id + '@x.cl', phone: '+56 9 1111 2222', waveCode: null, wmsCode: 'W' + id, profilesCount: 2, active: true, createdAt: '2026-10-14T13:00:00Z', updatedAt: '2026-10-14T13:00:00Z', ...extra });
const user = (role, permissions = []) => ({ user: { id: 1, role, permissions, name: 'X', allCompanies: true, companies: [] } });
const settle = async () => { for (let i = 0; i < 8; i++) await act(async () => { await new Promise((r) => setTimeout(r, 15)); }); };
const click = async (e) => { expect(e, 'no encontrado').toBeTruthy(); await act(async () => e.click()); await settle(); };
const setValue = async (el, v) => {
  const proto = el.tagName === 'SELECT' ? HTMLSelectElement : el.tagName === 'TEXTAREA' ? HTMLTextAreaElement : HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(proto.prototype, 'value').set.call(el, v); el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input', { bubbles: true })); });
};
const button = (root, text) => [...root.querySelectorAll('button, a')].find((b) => b.textContent.trim() === text);

describe('RUT y olas', () => {
  it('valida el dígito verificador y da formato', () => {
    expect(normalizeRut('760864285')).toBe('76.086.428-5');
    expect(normalizeRut('76.123.456-0')).toBe('76.123.456-0');
    expect(normalizeRut('10.000.013-k')).toBe('10.000.013-K');
    expect(normalizeRut('76.123.456-7')).toBeNull();
    expect(normalizeRut('SEED-1')).toBeNull();
  });
  it('«General (según reglas)» siempre primero; el código actual no se pierde', () => {
    expect(waveOptions([{ code: 'AM', name: 'AM' }], 'PM').map((o) => o.value)).toEqual(['general', 'AM', 'PM']);
  });
});

describe('Administración clientes › Empresas', () => {
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
    h.calls.length = 0; h.downloads.length = 0; h.failCreate = null;
    h.companies = [company(1), company(2, { name: 'Montaña Sur', active: false, waveCode: 'PM', email: '' })];
    h.session = user('admin', ['gestionar_perfiles']);
    host = document.createElement('div'); document.body.append(host); root = createRoot(host);
  });
  afterEach(async () => { await act(async () => root.unmount()); host.remove(); document.body.innerHTML = ''; });

  it('columnas, estado con texto y ola «General (según reglas)»', async () => {
    await mount('tab=empresas');
    expect(host.querySelector('.page-header h1').textContent).toBe('Administración clientes');
    expect([...host.querySelectorAll('.view-tabs a')].map((a) => a.textContent)).toEqual(['Perfiles', 'Empresas', 'Solicitudes']);
    expect([...host.querySelectorAll('.clients-list th')].map((t) => t.textContent)).toEqual(['', 'Nombre empresa', 'RUT', 'Correo', 'Teléfono', 'Ola', 'N° perfiles', 'Estado', 'Fecha de creación']);
    const rows = [...host.querySelectorAll('.clients-list tbody tr')].map((r) => [...r.querySelectorAll('td')].slice(1, 8).map((c) => c.textContent));
    expect(rows[0]).toEqual(['Empresa 1', '76.123.456-0', 'e1@x.cl', '+56 9 1111 2222', 'General (según reglas)', '2', 'Activa']);
    expect(rows[1]).toEqual(['Montaña Sur', '76.123.456-0', '—', '+56 9 1111 2222', 'PM', '2', 'Inactiva']);
    expect(h.calls.some(([, p]) => p.startsWith('/v1/companies?') && p.includes('limit=20'))).toBe(true);
  });

  it('panel: solo el botón que corresponde al estado; Ver integraciones con la empresa', async () => {
    await mount('tab=empresas&empresa=1');
    const panel = host.querySelector('.clients-panel');
    expect(panel.querySelector('h2').textContent).toBe('Detalle de empresa Empresa 1');
    expect(panel.textContent).toContain('Código Invas');
    expect(button(panel, 'Ver integraciones').getAttribute('href')).toBe('/integraciones?empresa=1');
    expect(button(panel, 'Desactivar')).toBeTruthy();
    expect(button(panel, 'Activar')).toBeUndefined();
    await act(async () => root.unmount()); root = createRoot(host);
    await mount('tab=empresas&empresa=2');
    const p2 = host.querySelector('.clients-panel');
    expect(button(p2, 'Activar')).toBeTruthy();
    expect(button(p2, 'Desactivar')).toBeUndefined();
  });

  it('sin «Gestionar perfiles»: lectura (sin Nueva empresa, Editar, Activar ni Desactivar)', async () => {
    h.session = user('admin');
    await mount('tab=empresas&empresa=1');
    expect(button(host, 'Nueva empresa')).toBeUndefined();
    const panel = host.querySelector('.clients-panel');
    for (const t of ['Editar', 'Desactivar', 'Activar']) expect(button(panel, t)).toBeUndefined();
    expect(button(panel, 'Ver integraciones')).toBeTruthy();
    expect(button(host, 'Excel acciones')).toBeTruthy();
  });

  it('el código Invas solo para el Administrador', async () => {
    h.session = user('jefe_operaciones', ['gestionar_perfiles']);
    h.companies = [company(1, { wmsCode: undefined })];
    await mount('tab=empresas&empresa=1');
    expect(host.querySelector('.clients-panel').textContent).not.toContain('Código Invas');
  });

  it('Nueva empresa: obligatorios de la BD y ola; «General» se guarda como null y el RUT con formato', async () => {
    await mount('tab=empresas');
    await click(button(host, 'Nueva empresa'));
    const modal = document.querySelector('.form-modal');
    await click(button(modal, 'Crear empresa'));
    const errs = [...modal.querySelectorAll('.modal-field-error')].map((e) => e.textContent);
    expect(errs).toEqual(['El nombre es obligatorio.', 'El RUT es obligatorio.', 'Elige la ola. Si no tiene una fija, elige «General (según reglas)».']);
    const [name, rut] = modal.querySelectorAll('input');
    await setValue(name, 'Andes Outdoor SpA');
    await setValue(rut, '76.123.456-7');
    await setValue(modal.querySelector('select'), 'general');
    await click(button(modal, 'Crear empresa'));
    expect(modal.textContent).toContain('RUT inválido');
    await setValue(rut, '760864285');
    await click(button(modal, 'Crear empresa'));
    const post = h.calls.find(([m, p]) => m === 'POST' && p === '/v1/companies');
    expect(post[2]).toMatchObject({ name: 'Andes Outdoor SpA', taxId: '76.086.428-5', waveCode: null, wmsCode: '' });
  });

  it('RUT repetido: el error del servidor queda en el campo', async () => {
    const { ApiError } = await import('@/services/api');
    h.failCreate = new ApiError('Ya existe una empresa con ese RUT.', 409, 'tax_id_taken', [{ field: 'taxId', message: 'Ya existe una empresa con ese RUT.' }]);
    await mount('tab=empresas');
    await click(button(host, 'Nueva empresa'));
    const modal = document.querySelector('.form-modal');
    const [name, rut] = modal.querySelectorAll('input');
    await setValue(name, 'Repetida');
    await setValue(rut, '11.111.111-1');
    await setValue(modal.querySelector('select'), 'AM');
    await click(button(modal, 'Crear empresa'));
    expect(rut.getAttribute('aria-invalid')).toBe('true');
    expect(modal.querySelector('.modal-field-error').textContent).toBe('Ya existe una empresa con ese RUT.');
  });

  it('Desactivar pide motivo y avisa que sus usuarios dejan de operar', async () => {
    await mount('tab=empresas&empresa=1');
    await click(button(host.querySelector('.clients-panel'), 'Desactivar'));
    const modal = document.querySelector('.form-modal');
    expect(modal.textContent).toContain('dejarán de operar');
    await click(button(modal, 'Desactivar'));
    expect(modal.textContent).toContain('Indica el motivo de la desactivación.');
    await setValue(modal.querySelector('textarea'), 'Término de contrato');
    await click(button(modal, 'Desactivar'));
    expect(h.calls.find(([m, p]) => m === 'POST' && p === '/v1/companies/1/disable')[2]).toEqual({ reason: 'Término de contrato' });
  });

  it('Excel: exportar las seleccionadas o por filtros, desde el Core', async () => {
    await mount('tab=empresas&status=inactive');
    await act(async () => host.querySelector('tbody input[type=checkbox]').click());
    await click(button(host, 'Excel acciones'));
    await click(button(host, 'Exportar seleccionadas (1)'));
    await click(button(host, 'Excel acciones'));
    await click(button(host, 'Exportar empresas (filtros actuales)'));
    expect(h.downloads).toEqual([['/v1/companies/export?ids=1', 'empresas.xlsx', 'core'], ['/v1/companies/export?status=inactive', 'empresas.xlsx', 'core']]);
  });
});
