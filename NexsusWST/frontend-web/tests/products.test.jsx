import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';

// Tarea 09 · Fase 1: página Productos (patrón de Clientes › Todos los perfiles).
const h = vi.hoisted(() => ({ session: null, calls: [], products: [], detail: {}, created: null, downloads: [], importResult: null }));
vi.mock('@/state/auth', () => ({ ROLE_LABELS: {}, useAuth: () => ({ session: h.session, companyId: null, logout: () => {} }) }));
vi.mock('@/services/api', () => {
  class ApiError extends Error { constructor(m, status, code, fields = []) { super(m); this.status = status; this.code = code; this.fields = fields; } }
  return {
    ApiError, apiErrorMessage: (e) => e.message, withCompany: (p) => p, downloadFile: async (path, name, service) => { h.downloads.push([path, name, service]); },
    api: async (service, path, init) => {
      const body = init?.body instanceof FormData ? 'form' : init?.body ? JSON.parse(init.body) : undefined;
      if (path === '/v1/products/import') return h.importResult;
      h.calls.push([init?.method ?? 'GET', path, body]);
      if (path === '/v1/clients') return { data: [{ id: 1, name: 'Andes', active: true }, { id: 2, name: 'Montaña Sur', active: false }] };
      if (path === '/v1/waves') return { data: [{ id: 'w1', code: 'AM', name: 'Mañana', type: '' }] };
      if (!init?.method && path.startsWith('/v1/products?')) {
        if (path.includes('type=simple')) return { data: [product(1), product(2, { sku: 'AO-2' })], total: 2, limit: 20, offset: 0 };
        return { data: h.products, total: h.products.length, limit: 20, offset: 0 };
      }
      const m = path.match(/^\/v1\/products\/(\d+)$/);
      if (m && !init?.method) return h.detail[m[1]];
      if (init?.method === 'POST' && path === '/v1/products') return { ...product(50), sku: body.sku.toUpperCase(), type: body.type };
      return { data: [] };
    },
  };
});

const product = (id, extra = {}) => ({ id, companyId: 1, companyName: 'Andes', sku: 'AO-' + id, description: 'Producto ' + id, type: 'simple', active: true, waveCode: null,
  createdAt: '2026-10-14T13:00:00Z', updatedAt: '2026-11-01T13:00:00Z', updatedBy: 'Admin Demo', ...extra });
const user = (role, companies = []) => ({ user: { id: 1, role, permissions: [], name: 'X', allCompanies: role !== 'cliente', companies } });
const settle = async () => { for (let i = 0; i < 8; i++) await act(async () => { await new Promise((r) => setTimeout(r, 15)); }); };
const click = async (e) => { expect(e, 'no encontrado').toBeTruthy(); await act(async () => e.click()); await settle(); };
const setValue = async (el, v) => {
  const proto = el.tagName === 'SELECT' ? HTMLSelectElement : HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(proto.prototype, 'value').set.call(el, v); el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input', { bubbles: true })); });
};
const button = (root, text) => [...root.querySelectorAll('button, a')].find((b) => b.textContent.trim() === text);
const field = (root, label) => { const l = [...root.querySelectorAll('label')].find((x) => x.textContent.trim() === label); return l && root.querySelector('#' + CSS.escape(l.htmlFor)); };

describe('Productos', () => {
  let root, host;
  const mount = async (search) => {
    globalThis.__nav = { params: new URLSearchParams(search), calls: [], pathname: '/productos' };
    const { ProductsView } = await import('@/features/products');
    await act(async () => root.render(<ProductsView />));
    await settle();
  };
  beforeEach(() => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
    globalThis.matchMedia = () => ({ matches: false, addEventListener() {}, removeEventListener() {} });
    h.calls.length = 0;
    h.products = [product(1, { waveCode: 'AM' }), product(9, { sku: 'AO-KIT', type: 'kit', description: 'Kit trekking' }), product(3, { active: false })];
    h.detail = {
      9: { ...h.products[1], components: [{ productId: 1, sku: 'AO-1', description: 'Producto 1', quantity: 2 }] },
      1: h.products[0],
    };
    h.session = user('admin');
    host = document.createElement('div'); document.body.append(host); root = createRoot(host);
  });
  afterEach(async () => { await act(async () => root.unmount()); host.remove(); document.body.innerHTML = ''; });

  it('tabla: columnas, chip de tipo, ola «General» y filtros en la URL van al servidor', async () => {
    await mount('q=ao&tipo=kit&estado=active&ola=general&cliente=1');
    expect([...host.querySelectorAll('.clients-list th')].map((t) => t.textContent)).toEqual(['', 'SKU', 'Descripción', 'Tipo', 'Cliente', 'Ola', 'Estado', 'Fecha de actualización']);
    const rows = [...host.querySelectorAll('.clients-list tbody tr')].map((r) => [...r.querySelectorAll('td')].slice(1, 7).map((c) => c.textContent));
    expect(rows[0]).toEqual(['AO-1', 'Producto 1', 'Artículo', 'Andes', 'AM', 'Activo']);
    expect(rows[1].slice(0, 5)).toEqual(['AO-KIT', 'Kit trekking', 'Kit', 'Andes', 'General (según reglas)']);
    const list = h.calls.find(([m, p]) => m === 'GET' && p.startsWith('/v1/products?'))[1];
    for (const part of ['q=ao', 'type=kit', 'status=active', 'wave=general', 'company_id=1']) expect(list).toContain(part);
    expect(host.querySelector('.orders-search input').placeholder).toBe('Buscar SKU o descripción');
  });

  it('panel de un kit: composición, nota de stock y quién lo modificó', async () => {
    await mount('producto=9');
    const panel = host.querySelector('.clients-panel');
    expect(panel.querySelector('h2').textContent).toBe('Detalle de producto AO-KIT');
    expect(panel.textContent).toContain('Kit: no tiene stock propio.');
    expect([...panel.querySelectorAll('.compact-table tbody td')].map((c) => c.textContent)).toEqual(['AO-1', 'Producto 1', '2']);
    expect(panel.textContent).toContain('Admin Demo');
    expect(panel.textContent.toLowerCase()).not.toContain('stock disponible');
    expect(button(panel, 'Desactivar')).toBeTruthy();
    expect(button(panel, 'Activar')).toBeUndefined();
    expect(button(panel, 'Editar')).toBeTruthy();
  });

  it('Jefe y Cliente: solo lectura; el Cliente no filtra por cliente', async () => {
    h.session = user('jefe_operaciones');
    await mount('producto=1');
    expect(button(host, 'Nuevo producto')).toBeUndefined();
    for (const t of ['Editar', 'Desactivar']) expect(button(host.querySelector('.clients-panel'), t)).toBeUndefined();
    await act(async () => root.unmount()); root = createRoot(host);
    h.session = user('cliente', [1]);
    h.calls.length = 0;
    await mount('');
    expect(button(host, 'Nuevo producto')).toBeUndefined();
    expect(h.calls.some(([, p]) => p === '/v1/clients')).toBe(false);
    await click(button(host, 'Filtros'));
    expect(document.querySelector('.modal').textContent).not.toContain('Cliente');
  });

  it('Nuevo producto: obligatorios, kit con componentes del mismo cliente y ola General = null', async () => {
    await mount('');
    await click(button(host, 'Nuevo producto'));
    const modal = document.querySelector('.form-modal');
    await click(button(modal, 'Crear producto'));
    expect([...modal.querySelectorAll('.modal-field-error')].map((e) => e.textContent)).toEqual(['Elige el cliente.', 'El SKU es obligatorio.', 'La descripción es obligatoria.']);
    expect([...field(modal, 'Cliente *').options].map((o) => o.textContent)).toEqual(['Elige el cliente', 'Andes']); // solo clientes activos
    await setValue(field(modal, 'Cliente *'), '1');
    await setValue(field(modal, 'SKU *'), 'ao-kit-2');
    await setValue(field(modal, 'Descripción *'), 'Kit de prueba');
    await setValue(field(modal, 'Tipo *'), 'kit');
    expect(modal.textContent).toContain('Kit: no tiene stock propio.');
    // El buscador pide solo artículos activos del cliente elegido.
    expect(h.calls.some(([, p]) => p.startsWith('/v1/products?') && p.includes('company_id=1') && p.includes('type=simple') && p.includes('status=active'))).toBe(true);
    await setValue(modal.querySelector('input[aria-label="SKU del componente 1"]'), 'ao-1');
    await setValue(modal.querySelector('input[aria-label="Cantidad del componente 1"]'), '2');
    await click(button(modal, 'Crear producto'));
    const post = h.calls.find(([m, p]) => m === 'POST' && p === '/v1/products');
    expect(post[2]).toEqual({ companyId: 1, sku: 'ao-kit-2', description: 'Kit de prueba', type: 'kit', waveCode: null, components: [{ sku: 'AO-1', quantity: 2 }] });
  });

  it('Editar: cliente, SKU y tipo fijos', async () => {
    await mount('producto=1');
    await click(button(host.querySelector('.clients-panel'), 'Editar'));
    const modal = document.querySelector('.form-modal');
    for (const l of ['Cliente *', 'SKU *', 'Tipo *']) expect(field(modal, l).disabled).toBe(true);
    expect(field(modal, 'Ola').value).toBe('AM');
  });

  it('menú Excel: plantilla e importación solo para el Administrador; exportar seleccionados o con filtros', async () => {
    h.downloads.length = 0;
    await mount('tipo=kit');
    await click(button(host, 'Excel acciones'));
    expect([...document.querySelectorAll('[role=menuitem]')].map((b) => b.textContent)).toEqual(['Descargar plantilla', 'Importar productos', 'Exportar seleccionados', 'Exportar (filtros actuales)']);
    await click(button(document.body, 'Exportar (filtros actuales)'));
    expect(h.downloads.at(-1)).toEqual(['/v1/products/export?type=kit', 'productos.xlsx', 'core']);
    await act(async () => root.unmount()); root = createRoot(host);
    h.session = user('cliente', [1]);
    await mount('');
    await click(button(host, 'Excel acciones'));
    expect([...document.querySelectorAll('[role=menuitem]')].map((b) => b.textContent)).toEqual(['Exportar seleccionados', 'Exportar (filtros actuales)']);
  });

  it('Importar productos: reutiliza el resultado de la carga masiva con fila y motivo', async () => {
    h.importResult = { rows: 3, created: [{ row: 2, id: 60, name: 'AO-60 · Mochila' }], errors: [{ row: 3, field: 'sku', message: 'Ese SKU ya existe para el cliente.' }, { row: 4, field: 'companyId', message: 'No hay un cliente con el RUT 9.' }] };
    await mount('');
    await click(button(host, 'Excel acciones'));
    await click(button(document.body, 'Importar productos'));
    const modal = document.querySelector('.form-modal');
    expect(modal.textContent).toContain('Solo artículos');
    const input = modal.querySelector('input[type=file]');
    await act(async () => {
      Object.defineProperty(input, 'files', { value: [new File(['x'], 'productos.xlsx')], configurable: true });
      input.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await click(button(modal, 'Importar'));
    const result = document.querySelector('.form-modal');
    expect(result.textContent).toContain('1 producto creado');
    expect([...result.querySelectorAll('tbody tr')].map((r) => r.textContent)).toEqual(['3Ese SKU ya existe para el cliente.', '4No hay un cliente con el RUT 9.']);
  });
});
