import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act} from 'react';
const h=vi.hoisted(()=>{
  class ApiError extends Error{constructor(m,s,c,f=[],r){super(m);this.status=s;this.code=c;this.fields=f;this.ref=r}}
  return {ApiError,session:{user:{id:2,role:'jefe_operaciones',permissions:['despacho'],name:'J',allCompanies:true,companies:[]}},s:{}};
});
const {s,ApiError}=h;
vi.mock('@/state/auth',()=>({ROLE_LABELS:{},useAuth:()=>({session:h.session,companyId:null,logout:()=>{}})}));
vi.mock('@/services/orderReference',()=>({productsForCompany:async()=>[{sku:'AO-1',description:'Mochila Wilson'},{sku:'AO-2',description:'Carpa'}]}));
vi.mock('@/services/api',()=>({ApiError,apiErrorMessage:e=>e.message,downloadFile:async()=>{},printPdf:async()=>{},saveBlob:()=>{},wsUrl:p=>'ws://x'+p,withCompany:p=>p,api:async(_s,path,init)=>{
  if(path==='/v1/ws-ticket')return {ticket:'t'};
  const method=init?.method??'GET';const body=init?.body?JSON.parse(init.body):undefined;s.calls.push({path,method,body});await new Promise(r=>setTimeout(r,2));
  if(method==='POST'&&/\/incident$/.test(path)){if(s.fail){const f=s.fail;s.fail=null;throw f}const id=Number(path.split('/')[3]);s.orders=s.orders.filter(o=>o.id!==id);return {}}
  if(path==='/v1/dispatch/couriers')return {data:[{courier:'blueexpress',orders:s.orders.length,packages:3}]};
  if(path.startsWith('/v1/dispatch/manifests'))return {data:[]};
  if(path.startsWith('/v1/orders?'))return {data:s.orders,total:s.orders.length};
  const m=path.match(/^\/v1\/orders\/(\d+)$/);if(m)return {order:{},events:[],documents:[],packages:(s.pk[m[1]]||[]).map((c,i)=>({id:i,code:c,manifestId:null}))};
  if(path.includes('clients'))return {data:[{id:7,name:'Cliente 2',active:true}]};
  if(path.includes('catalogs/couriers'))return {data:[{code:'blueexpress',name:'BlueExpress'}]};
  return {data:[]};
}}));
const mk=(id,extra={})=>({id,externalRef:'4825776922'+id,companyId:7,courier:'blueexpress',status:'ready_pickup',source:'webhook',channel:'mercadolibre',recipientName:'Felipe Sandoval',recipientPhone:'+56 9 1234 5678',addressLine:'Los militares 5225',commune:'Las Condes',customerNotes:'Dejar en conserje',packagesCount:2,version:1,
  items:[{sku:'AO-1',quantity:1},{sku:'AO-2',quantity:1}],label:{strategy:'api',status:'ready',trackingCode:'BX-0'+id},...extra});
const sockets=[];class FakeWS{constructor(){sockets.push(this);setTimeout(()=>this.onopen?.(),0)}close(){}}
const settle=async()=>{for(let i=0;i<8;i++)await act(async()=>{await new Promise(r=>setTimeout(r,20))})};
const click=async e=>{expect(e,'no encontrado').toBeTruthy();await act(async()=>e.click());await settle()};
const byText=(t,r=document.body)=>[...r.querySelectorAll('button')].find(b=>b.textContent.trim()===t||b.getAttribute('aria-label')===t);
const setValue=async(el,v)=>{const proto=el.tagName==='TEXTAREA'?HTMLTextAreaElement:HTMLSelectElement;await act(async()=>{Object.getOwnPropertyDescriptor(proto.prototype,'value').set.call(el,v);el.dispatchEvent(new Event(el.tagName==='SELECT'?'change':'input',{bubbles:true}))})};
const submit=async f=>{await act(async()=>f.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true})));await settle()};
const dialogs=()=>[...document.querySelectorAll('[role="dialog"]')];
describe('Retiro Fase 2: detalle e «Informar problema»',()=>{
  let root,host;
  const mount=async()=>{const {DispatchView}=await import('@/features/dispatch');const {Toaster}=await import('@/components/Toast');await act(async()=>root.render(<><DispatchView/><Toaster/></>));await settle()};
  const view=async id=>click(byText(`Ver orden 4825776922${id}`,host));
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;globalThis.WebSocket=FakeWS;sockets.length=0;sessionStorage.clear();h.session.user.permissions=['despacho'];
    Object.assign(s,{calls:[],fail:null,orders:[mk(0),mk(1)],pk:{1:['NX-1-1','NX-1-2'],2:['NX-2-1','NX-2-2']}});
    s.orders[0].id=1;s.orders[1].id=2;
    globalThis.__nav={params:new URLSearchParams('courier=blueexpress'),calls:[]};host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});

  it('VER abre el detalle con los datos del mockup y la columna Bulto en «—»',async()=>{
    await mount();await view(0);
    const d=dialogs()[0];expect(d.querySelector('h2,h1,.modal-title')?.textContent??d.textContent).toContain('BX-00 • #48257769220');
    for(const x of ['Cliente 2','Felipe Sandoval','+56 9 1234 5678','Los militares 5225','Las Condes','Mercado Libre','BlueExpress','Dejar en conserje'])expect(d.textContent).toContain(x);
    expect([...d.querySelectorAll('th')].map(t=>t.textContent)).toEqual(['Producto','SKU','Cantidad','Bulto']);
    const tr=[...d.querySelectorAll('tbody tr')];expect(tr.map(r=>r.textContent)).toEqual(['Mochila WilsonAO-11—','CarpaAO-21—']);
    expect(d.textContent).not.toContain('Cambiado de');
    await click(byText('Cerrar',d));expect(dialogs().length).toBe(0);
  });
  it('Informar problema: tipos, bulto o producto, motivo obligatorio; sale de la lista',async()=>{
    await mount();await view(1);
    await click([...dialogs()[0].querySelectorAll('tbody tr')][1]);
    await click(byText('Informar problema',dialogs()[0]));
    const d=dialogs().at(-1);const [kind,target]=d.querySelectorAll('select');
    expect([...kind.options].map(o=>o.textContent)).toEqual(['Courier no llega a la dirección','Otro problema en el retiro']);
    expect([...target.options].map(o=>o.textContent)).toEqual(['Toda la orden','Bulto NX-2-1','Bulto NX-2-2','Mochila Wilson (SKU AO-1)','Carpa (SKU AO-2)']);
    expect(target.value).toBe('sku:AO-2');
    await submit(d.querySelector('form'));expect(d.textContent).toContain('Indica el motivo');expect(s.calls.some(c=>/incident$/.test(c.path))).toBe(false);
    await setValue(target,'bulto:NX-2-2');await setValue(kind,'retiro');await setValue(d.querySelector('textarea'),'Caja rota');await submit(d.querySelector('form'));
    expect(s.calls.find(c=>/incident$/.test(c.path))).toEqual({path:'/v1/orders/2/incident',method:'POST',body:{kind:'retiro',reason:'Bulto NX-2-2: Caja rota'}});
    expect(dialogs().length).toBe(0);
    expect([...document.querySelectorAll('.toast-warning')].map(t=>t.textContent).join()).toContain('pasó a Incidencia');
    expect([...host.querySelectorAll('.dispatch-table tbody tr')].length).toBe(1);
  });
  it('Toda la orden: el motivo va sin prefijo y el kind por defecto es cobertura',async()=>{
    await mount();await view(0);await click(byText('Informar problema',dialogs()[0]));
    const d=dialogs().at(-1);await setValue(d.querySelector('textarea'),'Comuna sin cobertura');await submit(d.querySelector('form'));
    expect(s.calls.find(c=>/incident$/.test(c.path)).body).toEqual({kind:'courier_cobertura',reason:'Comuna sin cobertura'});
  });
  it('Error del servidor: se muestra en el diálogo y la orden sigue',async()=>{
    s.fail=new ApiError('No se puede reportar una incidencia en una orden «Entregada».',422,'invalid_transition');
    await mount();await view(0);await click(byText('Informar problema',dialogs()[0]));
    const d=dialogs().at(-1);await setValue(d.querySelector('textarea'),'x');await submit(d.querySelector('form'));
    expect(d.textContent).toContain('Entregada');expect([...host.querySelectorAll('.dispatch-table tbody tr')].length).toBe(2);
  });
  it('Sin permiso Despacho: el detalle se ve, pero sin «Informar problema»',async()=>{
    h.session.user.permissions=[];await mount();await view(0);
    expect(byText('Informar problema',dialogs()[0])).toBeUndefined();expect(byText('Cerrar',dialogs()[0])).toBeTruthy();
  });
  it('Si otra persona saca la orden mientras se mira, el detalle se cierra',async()=>{
    await mount();await view(0);s.orders=s.orders.filter(o=>o.id!==1);
    await act(async()=>sockets.at(-1).onmessage({data:JSON.stringify({type:'order.changed',orderId:1})}));await settle();await settle();
    expect(dialogs().length).toBe(0);
  });
});
