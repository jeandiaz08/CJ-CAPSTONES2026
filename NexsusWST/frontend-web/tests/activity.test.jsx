import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act} from 'react';
const h=vi.hoisted(()=>({session:{user:{id:2,role:'jefe_operaciones',permissions:[],name:'J',allCompanies:true,companies:[]}},calls:[],downloads:[],items:[],registry:[]}));
vi.mock('@/state/auth',()=>({ROLE_LABELS:{admin:'Administrador',jefe_operaciones:'Jefe de Operaciones',cliente:'Cliente',empaque:'Empaque',analista:'Analista'},useAuth:()=>({session:h.session,companyId:null,logout:()=>{}})}));
vi.mock('@/services/api',()=>({ApiError:class extends Error{},apiErrorMessage:e=>e.message,wsUrl:p=>'ws://x'+p,withCompany:(p,c)=>c==null?p:p+(p.includes('?')?'&':'?')+'company_id='+c,
  downloadFile:async(path,name)=>{h.downloads.push([path,name])},
  api:async(_s,path)=>{
    h.calls.push(path);
    if(path==='/v1/ws-ticket')return {ticket:'t'};
    if(path.startsWith('/v1/activity'))return {items:h.items,total:h.items.length};
    if(path.startsWith('/v1/orders/registry'))return {data:h.registry,total:h.registry.length};
    return {data:[]};
  }}));
const item=(id,extra={})=>({id,occurredAt:'2026-11-07T10:00:00-03:00',actor:{id:2,name:'Jefe Demo',role:'jefe_operaciones',kind:'human'},module:'orders',action:'validate',actionLabel:'Validada',
  entity:{type:'order',id:'15',label:'48257769215'},companyName:'Tienda Uno',reason:'Stock disponible',details:{},statusChange:'En validación → Validada',detailLines:['Ola: OLA-AM'],...extra});
const sockets=[];class FakeWS{constructor(){sockets.push(this);setTimeout(()=>this.onopen?.(),0)}close(){}}
const settle=async()=>{for(let i=0;i<8;i++)await act(async()=>{await new Promise(r=>setTimeout(r,15))})};
const click=async e=>{expect(e,'no encontrado').toBeTruthy();await act(async()=>e.click());await settle()};
const nav=(search)=>{globalThis.__nav={params:new URLSearchParams(search),calls:[],pathname:'/ordenes/registro'}};

describe('Registro de acciones',()=>{
  let root,host;
  const mount=async()=>{const {RegistryView}=await import('@/features/registry');await act(async()=>root.render(<RegistryView/>));await settle()};
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;globalThis.WebSocket=FakeWS;h.calls.length=0;h.downloads.length=0;h.items=[item('oe-1'),item('oe-2',{actor:{name:'Sistema',kind:'system'},actionLabel:'Orden creada'})];
    host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});

  it('Órdenes por defecto: chips, título con total, últimos 7 días y columnas del apartado',async()=>{
    nav('tab=acciones');await mount();
    expect([...host.querySelectorAll('.activity-chips button')].map(b=>[b.textContent,b.getAttribute('aria-pressed')])).toEqual([['Órdenes','true'],['Empaque','false'],['Retiro y entrega','false'],['Usuarios','false'],['Productos','false']]);
    expect(host.querySelector('.activity-title').textContent).toBe('Órdenes · 2 acciones');
    const call=new URLSearchParams(h.calls.find(c=>c.startsWith('/v1/activity?')).split('?')[1]);
    expect(call.get('module')).toBe('orders');expect(call.get('from')).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect((Date.parse(call.get('to'))-Date.parse(call.get('from')))/86400000).toBe(6);
    expect([...host.querySelectorAll('.activity-list th')].map(t=>t.textContent)).toEqual(['Fecha y hora','Actor','Perfil','Humano/Sistema','Acción','N° orden','Cliente','Estado anterior → nuevo','Motivo u observación','Detalle']);
    const cells=[...host.querySelectorAll('.activity-list tbody tr')][0].querySelectorAll('td');
    expect([...cells].slice(1).map(c=>c.textContent)).toEqual(['Jefe Demo','Jefe de Operaciones','Humano','Validada','48257769215','Tienda Uno','En validación → Validada','Stock disponible','Ola: OLA-AM']);
    expect(cells[5].querySelector('a').getAttribute('href')).toBe('/ordenes/15');
    expect(cells[8].title).toBe('Stock disponible');
  });
  it('Clic en la fila: se despliega el antes y el después; otro clic la cierra',async()=>{
    nav('tab=acciones');await mount();
    await click(host.querySelector('.activity-list tbody tr'));
    const d=host.querySelector('.activity-detail-row');expect(d.textContent).toContain('Ola');expect(d.textContent).toContain('OLA-AM');expect(d.textContent).toContain('En validación → Validada');
    await click(host.querySelector('.activity-list tbody tr'));expect(host.querySelector('.activity-detail-row')).toBeNull();
  });
  it('Apartado, búsqueda y filtros de la URL van al servidor; Retiro y entrega y Usuarios con sus columnas',async()=>{
    h.items=[item('mx-9',{module:'dispatch',action:'manifest_closed',actionLabel:'Cerró manifiesto (entrega)',entity:{type:'manifest',id:'9',label:'#9'},courierName:'Starken',transportCompany:'Transportes Sur',observation:'Faltó un cable',
      details:{orders:2,packages:3,pickerName:'Juan',pickerId:'11.111.111-1',vehiclePlate:'AB12CD'}})];
    nav('tab=acciones&section=dispatch&q=%239&from=2026-01-01&to=2026-01-31&actor=Jefe&role=jefe_operaciones&kind=human&page=1&limit=20');await mount();
    expect(h.calls).toContain('/v1/activity?module=dispatch&q=%239&from=2026-01-01&to=2026-01-31&actor=Jefe&role=jefe_operaciones&kind=human&limit=20&offset=20');
    const cells=[...host.querySelector('.activity-list tbody tr').querySelectorAll('td')].map(c=>c.textContent);
    expect(cells.slice(3)).toEqual(['Cerró manifiesto (entrega)','#9','Starken','2','3','Juan','11.111.111-1','AB12CD','Transportes Sur','Faltó un cable']);
    expect(host.querySelector('.activity-list tbody a').getAttribute('href')).toBe('/ordenes/registro?tab=acciones&section=dispatch&q=%239');
    expect(host.querySelector('button.btn.ghost').textContent).toContain('Filtros (4)');
    await click([...host.querySelectorAll('.activity-chips button')][3]);
    expect(globalThis.__nav.calls.at(-1)[1]).toContain('section=users');
    await click([...host.querySelectorAll('button')].find(b=>b.textContent==='Exportar registro'));
    expect(h.downloads.at(-1)).toEqual(['/v1/activity/export?module=dispatch&q=%239&from=2026-01-01&to=2026-01-31&actor=Jefe&role=jefe_operaciones&kind=human','registro-acciones-dispatch.xlsx']);
  });
  it('Usuarios: usuario y perfil afectados; sin acciones, un texto claro',async()=>{
    h.items=[item('ua-1',{module:'users',action:'updated',actionLabel:'Usuario editado',entity:{type:'user',id:'3',label:'Empaque Demo'},details:{targetRole:'empaque'},detailLines:['Perfil: Empaque → Jefe de Operaciones']})];
    nav('tab=acciones&section=users');await mount();
    expect([...host.querySelector('.activity-list tbody tr').querySelectorAll('td')].map(c=>c.textContent).slice(3)).toEqual(['Usuario editado','Empaque Demo','Empaque','Perfil: Empaque → Jefe de Operaciones']);
    h.items=[];await act(async()=>root.unmount());root=createRoot(host);await mount();
    expect(host.querySelector('.activity-list').textContent).toContain('Sin acciones de «Usuarios» para los filtros elegidos');
  });
});

describe('/ordenes/[id]',()=>{
  let root,host;
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;nav('');host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove()});
  it('abre Todas las órdenes con el detalle; si está en el registro, abre su registro',async()=>{
    const {OrderRedirect}=await import('@/features/orders');
    h.registry=[];await act(async()=>root.render(<OrderRedirect id={15}/>));await settle();
    expect(globalThis.__nav.calls).toContainEqual(['replace','/ordenes/todas/listado?order=15']);
    h.registry=[{id:42}];await act(async()=>root.render(<OrderRedirect id={42}/>));await settle();
    expect(globalThis.__nav.calls).toContainEqual(['replace','/ordenes/registro?order=42']);
  });
});
