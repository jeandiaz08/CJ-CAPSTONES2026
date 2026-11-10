import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import { PROVIDERS, supportedProviders } from '@/features/integrations/providers';

// Tarea 10 · Fase 1: Integraciones (tarjetas por proveedor y conexiones por empresa).
const h = vi.hoisted(() => ({ calls: [], integrations: [], fail: null }));
vi.mock('@/state/auth', () => ({ ROLE_LABELS: {}, useAuth: () => ({ session: { user: { id: 1, role: 'admin', permissions: [], name: 'X' } }, companyId: null, logout: () => {} }) }));
vi.mock('@/components/NotificationBell', () => ({ NotificationBell: () => null }));
vi.mock('@/services/api', () => {
  class ApiError extends Error { constructor(m, status, code, fields = []) { super(m); this.status = status; this.code = code; this.fields = fields; } }
  return {
    ApiError, apiErrorMessage: (e) => e.message, withCompany: (p) => p, downloadFile: async () => {},
    api: async (service, path, init) => {
      h.calls.push([init?.method ?? 'GET', path, init?.body ? JSON.parse(init.body) : undefined]);
      if (h.fail) throw h.fail;
      if (path === '/v1/integrations') return { data: h.integrations };
      if (path === '/v1/clients') return { data: [{ id: 1, name: 'Andes Outdoor SpA' }, { id: 2, name: 'Casa Bella Ltda' }] };
      const m = path.match(/^\/v1\/integrations\/(\d+)\/(enable|disable)$/);
      if (m) { h.integrations = h.integrations.map((i) => (i.id === Number(m[1]) ? { ...i, active: m[2] === 'enable' } : i)); return undefined; }
      return { data: [] };
    },
  };
});

const conn = (id, extra = {}) => ({ id, companyId: 1, provider: 'mercadolibre', kind: 'channel', channel: 'mercadolibre', defaultCourier: '', active: true,
  createdBy: 'user:7 Ana Pérez', createdAt: '2026-11-02T13:00:00Z', webhookPath: `/webhooks/${extra.provider ?? 'mercadolibre'}/${id}`, ...extra });
const settle = async () => { for (let i = 0; i < 8; i++) await act(async () => { await new Promise((r) => setTimeout(r, 15)); }); };
const click = async (e) => { expect(e, 'no encontrado').toBeTruthy(); await act(async () => e.click()); await settle(); };
const setValue = async (el, v) => {
  const proto = el.tagName === 'SELECT' ? HTMLSelectElement : HTMLInputElement;
  await act(async () => { Object.getOwnPropertyDescriptor(proto.prototype, 'value').set.call(el, v); el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input', { bubbles: true })); });
};
const button = (root, text) => [...root.querySelectorAll('button, a')].find((b) => b.textContent.trim() === text);
const card = (root, name) => [...root.querySelectorAll('.integration-card')].find((c) => c.querySelector('h3').textContent === name);

describe('Lista única de proveedores', () => {
  it('los soportados son los que acepta el backend; Shopify queda «Próximamente»', () => {
    expect(supportedProviders('channel').map((p) => p.code)).toEqual(['mercadolibre', 'generic']);
    expect(supportedProviders('courier').map((p) => p.code).sort()).toEqual(['blueexpress', 'chilexpress', 'generic', 'starken']);
    expect(PROVIDERS.find((p) => p.code === 'shopify').status).toBe('soon');
  });
});

describe('Integraciones', () => {
  let root, host;
  const mount = async (search = '') => {
    globalThis.__nav = { params: new URLSearchParams(search), calls: [], pathname: '/integraciones' };
    const { IntegrationsView } = await import('@/features/integrations');
    await act(async () => root.render(<IntegrationsView />));
    await settle();
  };
  const remount = async (search) => { await act(async () => root.unmount()); root = createRoot(host); await mount(search); };
  beforeEach(() => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
    globalThis.matchMedia = () => ({ matches: false, addEventListener() {}, removeEventListener() {} });
    h.calls.length = 0; h.fail = null;
    h.integrations = [
      conn(1), conn(2, { companyId: 2 }), conn(3, { companyId: 2, active: false }),
      conn(4, { provider: 'generic', channel: 'generic', defaultCourier: 'starken' }),
      conn(5, { provider: 'blueexpress', kind: 'courier', channel: 'blueexpress' }),
      conn(6, { provider: 'generic', kind: 'courier', channel: 'generic', companyId: 2 }),
    ];
    host = document.createElement('div'); document.body.append(host); root = createRoot(host);
  });
  afterEach(async () => { await act(async () => root.unmount()); host.remove(); document.body.innerHTML = ''; });

  it('tarjetas: secciones, un badge por estado y resumen por proveedor', async () => {
    await mount();
    expect([...host.querySelectorAll('.view-tabs a')].map((a) => [a.textContent, a.getAttribute('aria-current')])).toEqual([['Integraciones', 'page'], ['Clientes', null]]);
    expect([...host.querySelectorAll('section[aria-label]')].map((s) => s.getAttribute('aria-label'))).toEqual(['Canales', 'Couriers', 'Integraciones']);
    const badge = (name) => card(host, name).querySelector('.badge').textContent;
    expect(['Shopify', 'WooCommerce', 'Mercado Libre', 'API Nexus', 'Chilexpress', 'Mercado Envíos', 'Recíbelo', 'Invas WMS'].map(badge))
      .toEqual(['Próximamente', 'Próximamente', 'Disponible', 'Disponible', 'Disponible', 'Sin integración', 'Sin integración', 'Disponible']);
    expect(host.textContent).not.toContain('API mock');
    expect(card(host, 'Courier genérico')).toBeUndefined();

    const ml = card(host, 'Mercado Libre');
    expect(ml.querySelector('footer').textContent).toContain('2 conexiones activas · 1 inactiva');
    expect(button(ml, 'Ver conexiones').getAttribute('href')).toBe('/integraciones?tab=clientes&tipo=channel&proveedor=mercadolibre');
    // «generic» de canal y de courier no se mezclan.
    expect(card(host, 'API Nexus').querySelector('footer').textContent).toContain('1 conexión activa');
    expect(card(host, 'Chilexpress').querySelector('footer').textContent).toContain('Sin conexiones');
    expect(ml.querySelector('footer').textContent).toContain('Sincronizado: —');

    // Tarjetas fijas: sin enlace, sin interruptor ni engranaje.
    expect(card(host, 'Recíbelo').querySelector('footer').textContent).toBe('Buscar en oficina');
    expect(card(host, 'Mercado Envíos').querySelector('footer').textContent).toBe('Llega por el canal Mercado Libre');
    expect(card(host, 'Invas WMS').textContent).toContain('estados de preparación');
    expect(card(host, 'Invas WMS').querySelector('footer').textContent).toContain('Configurado en el servidor');
    expect(card(host, 'Shopify').classList.contains('is-soon')).toBe(true);
    expect(host.querySelectorAll('.integration-card button, .integration-card input').length).toBe(0);
    expect(button(card(host, 'Shopify'), 'Ver conexiones')).toBeUndefined();
  });

  it('tarjetas: buscar por nombre', async () => {
    await mount();
    await setValue(host.querySelector('.orders-search input'), 'star');
    expect([...host.querySelectorAll('.integration-card h3')].map((x) => x.textContent)).toEqual(['Starken']);
  });

  it('clientes: columnas (sin «Último evento» ni «Errores 24 h») y filas', async () => {
    await mount('tab=clientes');
    expect([...host.querySelectorAll('.connections-list th')].map((t) => t.textContent)).toEqual(['Empresa', 'Tipo', 'Proveedor', 'Estado', 'Creada']);
    const rows = [...host.querySelectorAll('.connections-list tbody tr')].map((r) => [...r.querySelectorAll('td')].map((c) => c.textContent));
    expect(rows.length).toBe(6);
    expect(rows[0].slice(0, 4)).toEqual(['Andes Outdoor SpA', 'Canal', 'Mercado Libre', 'Activa']);
    expect(rows[0][4]).toContain('Ana Pérez');
    expect(rows[0][4]).not.toContain('user:');
    expect(rows[2].slice(0, 4)).toEqual(['Casa Bella Ltda', 'Canal', 'Mercado Libre', 'Inactiva']);
    expect(rows[5].slice(1, 3)).toEqual(['Courier', 'Courier genérico']);
    expect(button(host, 'Conectar cliente')).toBeUndefined(); // llega en la Fase 2
  });

  it('clientes: ?empresa= sin pestaña abre Clientes filtrada; ?proveedor=, tipo, estado y búsqueda', async () => {
    await mount('empresa=2');
    expect(host.querySelector('.view-tabs a[aria-current]').textContent).toBe('Clientes');
    expect(host.querySelectorAll('.connections-list tbody tr').length).toBe(3);
    expect(button(host, 'Filtros (1)')).toBeTruthy();
    await remount('tab=clientes&tipo=channel&proveedor=generic');
    expect([...host.querySelectorAll('.connections-list tbody tr')].map((r) => r.querySelectorAll('td')[2].textContent)).toEqual(['API Nexus']);
    await remount('tab=clientes&estado=inactive');
    expect(host.querySelectorAll('.connections-list tbody tr').length).toBe(1);
    await remount('tab=clientes&q=casa');
    expect(host.querySelectorAll('.connections-list tbody tr').length).toBe(3);
    await remount('tab=clientes&q=blue');
    expect(host.querySelectorAll('.connections-list tbody tr').length).toBe(1);
    await remount('tab=clientes&empresa=1&proveedor=starken');
    expect(host.querySelector('.connections-list').textContent).toContain('Ninguna conexión coincide con los filtros');
  });

  it('clientes: aplicar filtros y limpiar escriben la URL', async () => {
    await mount('tab=clientes');
    await click(button(host, 'Filtros'));
    const selects = [...document.querySelectorAll('.filters-modal select')];
    await setValue(selects[0], '2'); await setValue(selects[1], 'courier'); await setValue(selects[3], 'active');
    expect([...selects[2].options].map((o) => o.textContent)).toEqual(['Todos', 'Chilexpress', 'BlueExpress', 'Starken', 'Courier genérico']);
    await click(button(document.body, 'Aplicar filtros'));
    const pushed = new URLSearchParams(globalThis.__nav.calls.at(-1)[1].split('?')[1]);
    expect(Object.fromEntries(pushed)).toEqual({ tab: 'clientes', empresa: '2', tipo: 'courier', estado: 'active' });
    await click(button(host, 'Limpiar filtros'));
    expect(globalThis.__nav.calls.at(-1)).toEqual(['push', '/integraciones?tab=clientes']);
  });

  it('panel: datos, URL completa del webhook, secreto enmascarado y solo el botón que corresponde', async () => {
    const copied = [];
    Object.defineProperty(globalThis.navigator, 'clipboard', { configurable: true, value: { writeText: async (t) => { copied.push(t); } } });
    await mount('tab=clientes&conexion=4');
    const panel = host.querySelector('.clients-panel');
    expect(panel.querySelector('h2').textContent).toBe('Andes Outdoor SpA · API Nexus');
    expect(panel.textContent).toContain('Courier por defecto');
    expect(panel.textContent).toContain('starken');
    const url = window.location.origin + '/webhooks/generic/4';
    expect(panel.querySelector('.secret-row code').textContent).toBe(url);
    await click(button(panel, 'Copiar'));
    expect(copied).toEqual([url]);
    expect(panel.querySelector('code[aria-label="Secreto oculto"]').textContent).toMatch(/^•+$/);
    expect(panel.textContent).toContain('Últimos eventos');
    expect(panel.textContent).toContain('Aún no disponible.');
    expect(button(panel, 'Desactivar')).toBeTruthy();
    expect(button(panel, 'Activar')).toBeUndefined();
    await remount('tab=clientes&conexion=3');
    expect(button(host.querySelector('.clients-panel'), 'Activar')).toBeTruthy();
    expect(button(host.querySelector('.clients-panel'), 'Desactivar')).toBeUndefined();
    await remount('tab=clientes&conexion=99');
    expect(host.querySelector('.clients-panel').textContent).toContain('Esta conexión no existe.');
  });

  it('desactivar y activar: con confirmación, llama al endpoint y recarga', async () => {
    await mount('tab=clientes&conexion=1');
    await click(button(host.querySelector('.clients-panel'), 'Desactivar'));
    expect(h.calls.some(([m]) => m === 'POST')).toBe(false); // todavía no: falta confirmar
    expect(document.body.textContent).toContain('Desactivar conexión Andes Outdoor SpA · Mercado Libre');
    await click([...document.querySelectorAll('.form-modal button')].find((b) => b.textContent.trim() === 'Desactivar'));
    expect(h.calls.filter(([m]) => m === 'POST')).toEqual([['POST', '/v1/integrations/1/disable', undefined]]);
    const panel = host.querySelector('.clients-panel');
    expect(panel.querySelector('.badge').textContent).toBe('Inactiva');
    await click(button(panel, 'Activar'));
    await click([...document.querySelectorAll('.form-modal button')].find((b) => b.textContent.trim() === 'Activar'));
    expect(h.calls.filter(([m]) => m === 'POST').at(-1)).toEqual(['POST', '/v1/integrations/1/enable', undefined]);
    expect(host.querySelector('.clients-panel .badge').textContent).toBe('Activa');
  });

  it('403 del servidor: se muestra el error y no hay filas', async () => {
    h.fail = new Error('Sin permiso para realizar esta acción.');
    await mount('tab=clientes');
    expect(host.querySelector('.connections-list [role="alert"]').textContent).toContain('Sin permiso');
    expect(host.querySelectorAll('.connections-list tbody tr').length).toBe(0);
  });
});
