import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act} from 'react';
const h=vi.hoisted(()=>({session:{user:{id:2,role:'jefe_operaciones',permissions:[],name:'J',allCompanies:true,companies:[]}},calls:[]}));
vi.mock('@/state/auth',()=>({ROLE_LABELS:{},useAuth:()=>({session:h.session,companyId:null,logout:()=>{}})}));
vi.mock('@/services/api',()=>({ApiError:class extends Error{},apiErrorMessage:e=>e.message,wsUrl:p=>'ws://x'+p,withCompany:p=>p,api:async(_s,path)=>{
  h.calls.push(path);
  if(path==='/v1/ws-ticket')return {ticket:'t'};
  const m=path.match(/^\/v1\/orders\/(\d+)$/);if(m)return {order:{externalRef:'4825'+m[1]},events:[],packages:[],documents:[]};
  return {data:[]};
}}));
const sockets=[];class FakeWS{constructor(){sockets.push(this);setTimeout(()=>this.onopen?.(),0)}close(){}}
const settle=async()=>{for(let i=0;i<6;i++)await act(async()=>{await new Promise(r=>setTimeout(r,15))})};
const send=async msg=>{await act(async()=>sockets.at(-1).onmessage({data:JSON.stringify(msg)}));await settle()};
const notification=(id,kind,title)=>({type:'notification',audience:'internal',orderId:1,companyId:7,notification:{id,kind,title,body:'Detalle '+id,orderId:1,createdAt:'2026-11-07T15:00:00Z'}});
describe('Campana: avisos del WebSocket en memoria',()=>{
  let root,host,feed;
  const mount=async()=>{
    feed=await import('@/hooks/useNotificationFeed');const {NotificationBell}=await import('@/components/NotificationBell');
    function App(){feed.useNotificationFeed();return <NotificationBell/>}
    await act(async()=>root.render(<App/>));await settle();
  };
  const bell=()=>host.querySelector('.notification-bell > button');
  const items=()=>[...host.querySelectorAll('.notification-popover li')].map(li=>li.querySelector('strong').textContent);
  beforeEach(async()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;globalThis.WebSocket=FakeWS;sockets.length=0;h.calls.length=0;host=document.createElement('div');document.body.append(host);root=createRoot(host);(await import('@/hooks/useNotificationFeed')).clearNotices()});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove()});

  it('sin avisos: sin contador y con un texto que explica qué aparece',async()=>{
    await mount();expect(bell().getAttribute('aria-label')).toBe('Notificaciones');expect(host.querySelector('.notification-count')).toBeNull();
    await act(async()=>bell().click());expect(host.querySelector('.notification-popover').textContent).toContain('Sin avisos por ahora');
  });
  it('incidencia, etiqueta disponible, despacho incompleto y entrega; lo demás no entra',async()=>{
    await mount();
    await send(notification(1,'incident','Orden #1 en Incidencia'));
    await send(notification(2,'label_ready','Etiqueta lista: orden #2'));
    await send(notification(3,'voided','Orden #3 Anulada'));
    await send(notification(4,'label_failed','Sin etiqueta'));
    await send(notification(5,'despacho_incompleto','Despacho incompleto: orden #5'));
    await send({type:'order.changed',orderId:9,companyId:7,status:'ready_pickup',version:2});
    await send({type:'order.changed',orderId:9,companyId:7,status:'handed_over',version:3});
    expect(h.calls).toContain('/v1/orders/9');
    expect(host.querySelector('.notification-count').textContent).toBe('4');expect(bell().getAttribute('aria-label')).toBe('Notificaciones, 4 sin leer');
    await act(async()=>bell().click());
    expect(items()).toEqual(['Orden #48259 entregada al courier','Despacho incompleto: orden #5','Etiqueta lista: orden #2','Orden #1 en Incidencia']);
    expect(host.querySelector('.notice-despacho_incompleto svg')).toBeTruthy();
    expect(host.querySelector('.notification-count')).toBeNull();
  });
  it('no repite un aviso y guarda solo los últimos 20',async()=>{
    await mount();
    await send(notification(1,'incident','Uno'));await send(notification(1,'incident','Uno'));
    await send({type:'order.changed',orderId:5,status:'handed_over'});await send({type:'order.changed',orderId:5,status:'handed_over'});
    expect(host.querySelector('.notification-count').textContent).toBe('2');
    for(let i=10;i<35;i++)await send(notification(i,'incident','Aviso '+i));
    await act(async()=>bell().click());expect(items().length).toBe(20);expect(items()[0]).toBe('Aviso 34');
  });
  it('se cierra con Escape o con un clic afuera',async()=>{
    await mount();await act(async()=>bell().click());expect(host.querySelector('.notification-popover')).toBeTruthy();
    await act(async()=>document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape'})));expect(host.querySelector('.notification-popover')).toBeNull();
    await act(async()=>bell().click());await act(async()=>document.body.dispatchEvent(new Event('pointerdown',{bubbles:true})));expect(host.querySelector('.notification-popover')).toBeNull();
  });
});
describe('Seguimiento en el detalle',()=>{
  let root,host;
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove()});
  const render=async tracking=>{const {TrackingSection}=await import('@/features/orders/components/TrackingSection');await act(async()=>root.render(<TrackingSection tracking={tracking}/>))};
  it('sin datos: «Sin seguimiento disponible», sin inventar estados',async()=>{
    await render(undefined);expect(host.textContent).toContain('Sin seguimiento disponible');expect(host.querySelector('table')).toBeNull();
    await render({available:false,events:[]});expect(host.textContent).toContain('Sin seguimiento disponible');
  });
  it('con datos: estado, origen y fecha',async()=>{
    await render({available:true,events:[{id:1,provider:'mercadoenvios',status:'in_transit',description:'En tránsito a Santiago',occurredAt:'2026-11-07T12:00:00Z'},{id:2,provider:'mercadoenvios',status:'delivered',occurredAt:'2026-11-07T18:00:00Z'}]});
    expect([...host.querySelectorAll('th')].map(t=>t.textContent)).toEqual(['Estado','Origen','Fecha']);
    const rows=[...host.querySelectorAll('tbody tr')].map(r=>[...r.querySelectorAll('td')].map(td=>td.textContent));
    expect(rows[0].slice(0,2)).toEqual(['in_transit · En tránsito a Santiago','mercadoenvios']);expect(rows[1][0]).toBe('delivered');expect(rows[0][2]).toMatch(/07-11-(20)?26/);
  });
});
