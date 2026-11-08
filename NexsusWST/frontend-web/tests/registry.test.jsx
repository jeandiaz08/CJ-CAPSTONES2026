import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act} from 'react';
import {canAccessPath,visibleMenu} from '@/state/menuAccess';
import {actorName} from '@/features/orders/format';
const h=vi.hoisted(()=>({session:{user:{id:2,role:'jefe_operaciones',permissions:[],name:'J',allCompanies:true,companies:[]}},calls:[],downloads:[],rows:[]}));
vi.mock('@/state/auth',()=>({ROLE_LABELS:{admin:'Administrador',jefe_operaciones:'Jefe de Operaciones',cliente:'Cliente',empaque:'Empaque',analista:'Analista'},useAuth:()=>({session:h.session,companyId:null,logout:()=>{}})}));
vi.mock('@/services/api',()=>({ApiError:class extends Error{},apiErrorMessage:e=>e.message,wsUrl:p=>'ws://x'+p,withCompany:(p,c)=>c==null?p:p+(p.includes('?')?'&':'?')+'company_id='+c,
  downloadFile:async(path,name)=>{h.downloads.push([path,name])},
  api:async(_s,path)=>{
    h.calls.push(path);
    if(path==='/v1/ws-ticket')return {ticket:'t'};
    if(path==='/v1/meta')return {statuses:[{code:'cancelled',label:'Cancelada'},{code:'handed_over',label:'Entregada al courier'},{code:'ready_pickup',label:'Lista para retiro'},{code:'pending',label:'Pendiente'},{code:'voided',label:'Anulada'}]};
    if(path.includes('clients'))return {data:[{id:7,name:'Tienda Uno',active:true}]};
    if(path.includes('catalogs/couriers'))return {data:[{code:'starken',name:'Starken'}]};
    if(path.startsWith('/v1/orders/registry')){const ids=new URLSearchParams(path.split('?')[1]).get('ids');const data=ids?h.rows.filter(r=>ids.split(',').includes(String(r.id))).concat(ids==='99'?[entry(99,{externalRef:'R99'})]:[]):h.rows;return {data,total:data.length}}
    const m=path.match(/^\/v1\/orders\/(\d+)$/);
    // El historial llega por id; la entrega quedó con un id menor que la creación.
    if(m)return {order:{},events:[{id:2,type:'status_change',event:'hand_over',fromStatus:'ready_pickup',toStatus:'handed_over',actor:'user:2 Jefe Demo',reason:'Retiro confirmado',occurredAt:'2025-09-07T20:47:00-03:00'},{id:5,type:'created',event:'create',fromStatus:'',toStatus:'pending',actor:'system:invas',reason:'',occurredAt:'2025-09-05T20:47:00-04:00'}],packages:[],documents:[]};
    return {data:[]};
  }}));
const entry=(id,extra={})=>({id,externalRef:'R'+id,companyId:7,companyName:'Tienda Uno',courier:'starken',courierName:'Starken',status:'handed_over',statusLabel:'Entregada al courier',previousStatus:'ready_pickup',previousStatusLabel:'Lista para retiro',closedAt:'2025-09-07T20:47:00-03:00',closedBy:{id:2,name:'Jefe Demo',role:'jefe_operaciones',kind:'human'},reason:'Retiro confirmado',motive:'Más de 1 año sin movimiento · Retiro confirmado',stale:true,...extra});
const sockets=[];class FakeWS{constructor(){sockets.push(this);setTimeout(()=>this.onopen?.(),0)}close(){}}
const settle=async()=>{for(let i=0;i<8;i++)await act(async()=>{await new Promise(r=>setTimeout(r,15))})};
const click=async e=>{expect(e,'no encontrado').toBeTruthy();await act(async()=>e.click());await settle()};
const u=(role,permissions=[])=>({role,permissions});

describe('Registro histórico: menú y rutas',()=>{
  it('submenú a /ordenes/registro para Admin, Jefe y Cliente; no para Empaque ni Analista',()=>{
    const kids=user=>visibleMenu(user).find(e=>e.name==='Órdenes')?.children?.map(c=>c.href)??[];
    for(const r of ['admin','jefe_operaciones','cliente'])expect(kids(u(r))).toContain('/ordenes/registro');
    for(const r of ['empaque','analista'])expect(kids(u(r))).not.toContain('/ordenes/registro');
    expect(canAccessPath(u('cliente'),'/ordenes/registro')).toBe(true);
    expect(canAccessPath(u('empaque',['despacho']),'/ordenes/registro')).toBe(false);
    expect(canAccessPath(u('jefe_operaciones'),'/ordenes/historial')).toBe(true);
  });
  it('actor de los eventos: usuario, proceso o nombre suelto',()=>{
    expect(actorName('user:2 Jefe Demo')).toBe('Jefe Demo');expect(actorName('user:9')).toBe('Usuario #9');
    for(const s of ['system:invas','Sistema','Invas',''])expect(actorName(s)).toBe('Sistema');
    expect(actorName('Marco Rojas')).toBe('Marco Rojas');
  });
});

describe('Registro histórico: «Registro órdenes»',()=>{
  let root,host;
  const mount=async()=>{const {RegistryView}=await import('@/features/registry');await act(async()=>root.render(<RegistryView/>));await settle()};
  const nav=(search)=>{globalThis.__nav={params:new URLSearchParams(search),calls:[],pathname:'/ordenes/registro'}};
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;globalThis.WebSocket=FakeWS;sockets.length=0;h.calls.length=0;h.downloads.length=0;
    h.session.user.role='jefe_operaciones';h.rows=[entry(1),entry(2,{externalRef:'R2',status:'cancelled',statusLabel:'Cancelada',previousStatus:'pending',previousStatusLabel:'Pendiente',stale:false,motive:'Compra desistida',closedBy:{name:'Sistema',kind:'system'}})];
    nav('');host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});

  it('Jefe: dos pestañas, columnas del mockup y una fila por orden',async()=>{
    await mount();
    expect([...host.querySelectorAll('.view-tabs a')].map(a=>a.textContent)).toEqual(['Registro órdenes','Registro de acciones']);
    expect([...host.querySelectorAll('.registry-list th')].map(t=>t.textContent)).toEqual(['','N° orden','Cliente','Cerrada por','Motivo','Estado','Fecha']);
    const cells=[...host.querySelectorAll('.registry-list tbody tr')][0].querySelectorAll('td');
    expect([...cells].slice(1,6).map(c=>c.textContent)).toEqual(['R1','Tienda Uno','Jefe Demo','Más de 1 año sin movimiento · Retiro confirmado','Entregada al courier']);
    expect(cells[3].title).toBe('Jefe Demo · Jefe de Operaciones');expect(cells[4].title).toBe('Más de 1 año sin movimiento · Retiro confirmado');
    expect(host.querySelector('.orders-pagination span').textContent).toBe('2 órdenes · 0 seleccionadas');
    expect(h.calls).toContain('/v1/orders/registry?limit=20&offset=0');
  });
  it('Cliente: solo «Registro órdenes», aunque la URL pida la otra pestaña',async()=>{
    h.session.user.role='cliente';nav('tab=acciones');await mount();
    expect([...host.querySelectorAll('.view-tabs a')].map(a=>a.textContent)).toEqual(['Registro órdenes']);
    expect(host.querySelector('.registry-list')).toBeTruthy();
  });
  it('Filtros de la URL van al servidor; clic en una fila deja la orden en la URL',async()=>{
    nav('q=R1&status=cancelled,voided&courier=starken&from=2025-01-01&to=2025-12-31&page=1&limit=50');await mount();
    expect(h.calls).toContain('/v1/orders/registry?q=R1&status=cancelled%2Cvoided&courier=starken&from=2025-01-01&to=2025-12-31&limit=50&offset=50');
    await click(host.querySelector('.registry-list tbody tr td:nth-child(3)'));
    expect(globalThis.__nav.calls.at(-1)[1]).toContain('order=1');
  });
  it('Detalle: datos del cierre y línea de tiempo ordenada por fecha (en tablet y teléfono, en un modal)',async()=>{
    nav('order=1');await mount();
    const d=document.querySelector('.modal.registry-modal');expect(d).toBeTruthy();
    expect(d.textContent).toContain('Registro orden #R1');
    const facts=Object.fromEntries([...d.querySelectorAll('.registry-facts>div')].map(x=>[x.querySelector('dt').textContent,x.querySelector('dd').textContent]));
    expect(facts).toMatchObject({Cliente:'Tienda Uno','Cerrada por':'Jefe Demo · Jefe de Operaciones','Estaba en':'Lista para retiro',Courier:'Starken',Motivo:'Más de 1 año sin movimiento · Retiro confirmado'});
    const items=[...d.querySelectorAll('.event-timeline li')];
    expect(items.map(li=>li.querySelector('strong').textContent)).toEqual(['Creada · Pendiente','Entregada al courier']);
    expect(items[0].textContent).toContain('Sistema');expect(items[1].textContent).toContain('Jefe Demo');expect(items[1].className).toBe('is-status');
  });
  it('Una orden que no está en la página se pide por id; si no está en el registro, se avisa',async()=>{
    nav('order=99');await mount();
    expect(h.calls).toContain('/v1/orders/registry?ids=99&limit=1&offset=0');
    expect(document.querySelector('.registry-modal').textContent).toContain('Registro orden #R99');
    await act(async()=>root.unmount());root=createRoot(host);
    nav('order=55');await mount();
    expect(document.querySelector('.registry-modal').textContent).toContain('Esta orden no está en el registro histórico.');
  });
  it('Excel acciones: las mismas que en Todas las órdenes sin importar, más «Exportar masivo»',async()=>{
    h.session.user.role='admin';nav('status=cancelled');await mount();
    const btn=t=>[...document.querySelectorAll('button')].find(b=>b.textContent.trim()===t);
    const menu=async()=>click(btn('Excel acciones'));
    await menu();
    expect([...document.querySelectorAll('.dropdown-menu button')].map(b=>b.textContent.trim())).toEqual(['Exportar seleccionadas','Exportar por filtros','Exportar masivo']);
    expect(btn('Exportar seleccionadas').disabled).toBe(true);
    await click(btn('Exportar masivo'));
    expect(h.downloads.at(-1)).toEqual(['/v1/orders/registry/export?status=cancelled','registro-ordenes.xlsx']);
    await act(async()=>host.querySelector('input[aria-label="Seleccionar orden R2"]').click());await settle();
    await menu();await click(btn('Exportar seleccionadas (1)'));
    expect(h.downloads.at(-1)).toEqual(['/v1/orders/registry/export?ids=2&format=xlsx','registro-ordenes.xlsx']);
    await menu();await click(btn('Exportar por filtros'));
    const d=document.querySelector('[role="dialog"]');expect(d.textContent).toContain('Exportar por filtros');
    const [from,to]=d.querySelectorAll('input[type="date"]');
    const setDate=async(el,v)=>act(async()=>{Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(el,v);el.dispatchEvent(new Event('input',{bubbles:true}));el.dispatchEvent(new Event('change',{bubbles:true}))});
    await setDate(from,'2025-01-01');await setDate(to,'2025-12-31');
    await act(async()=>[...d.querySelectorAll('input[type="radio"]')][1].click());
    await act(async()=>d.querySelector('form').dispatchEvent(new Event('submit',{bubbles:true,cancelable:true})));await settle();
    expect(h.downloads.at(-1)).toEqual(['/v1/orders/registry/export?from=2025-01-01&to=2025-12-31&format=csv&status=cancelled','registro-ordenes.csv']);
    expect(document.body.textContent).not.toContain('Importar órdenes');
  });
  it('Sin órdenes: un texto claro',async()=>{
    h.rows=[];await mount();expect(host.querySelector('.registry-list').textContent).toContain('No hay órdenes en el registro histórico');
  });
});
