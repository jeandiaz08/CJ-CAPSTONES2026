import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act} from 'react';
const h=vi.hoisted(()=>{
  class ApiError extends Error{constructor(m,s,c,f=[],r){super(m);this.status=s;this.code=c;this.fields=f;this.ref=r}}
  return {ApiError,session:{user:{id:2,role:'jefe_operaciones',permissions:['despacho'],name:'Jefe Demo',allCompanies:true,companies:[]}},s:{}};
});
const {s,ApiError}=h;
vi.mock('@/state/auth',()=>({ROLE_LABELS:{},useAuth:()=>({session:h.session,companyId:null,logout:()=>{}})}));
vi.mock('@/services/orderReference',()=>({productsForCompany:async()=>[{sku:'AO-1',description:'Mochila Wilson'}]}));
vi.mock('@/services/api',()=>({ApiError,apiErrorMessage:e=>e.message,downloadFile:async()=>{},printPdf:async()=>{},saveBlob:()=>{},wsUrl:p=>'ws://x'+p,withCompany:p=>p,api:async(_s,path,init)=>{
  if(path==='/v1/ws-ticket')return {ticket:'t'};
  const method=init?.method??'GET';const body=init?.body?JSON.parse(init.body):undefined;s.calls.push({path,method,body});await new Promise(r=>setTimeout(r,2));
  if(method==='POST'&&/\/close$/.test(path)){if(s.fail){const f=s.fail;s.fail=null;throw f}s.open.status='closed';s.closedList=[{...s.open,notes:body.notes,pickerName:body.pickerName,closedAt:'2026-11-07T15:00:00Z',closedBy:'user:2 Jefe Demo'}];s.open=null;return s.closeResult}
  if(path==='/v1/dispatch/couriers')return {data:[{courier:'blueexpress',orders:3,packages:5}]};
  if(path==='/v1/dispatch/manifests?status=open')return {data:s.open?[s.open]:[]};
  if(path==='/v1/dispatch/manifests?status=closed')return {data:s.closedList};
  if(path.startsWith('/v1/orders?'))return {data:s.orders,total:s.orders.length};
  const m=path.match(/^\/v1\/orders\/(\d+)$/);if(m)return {order:{incidentLabel:'Problema en el retiro',note:'Caja rota'},events:[],documents:[],packages:[]};
  if(path.includes('clients'))return {data:[{id:7,name:'Cliente 2',active:true}]};
  if(path.includes('catalogs/couriers'))return {data:[{code:'blueexpress',name:'BlueExpress'}]};
  return {data:[]};
}}));
const order=(id,extra={})=>({id,externalRef:'R'+id,companyId:7,courier:'blueexpress',status:'ready_pickup',commune:'Maipú',packagesCount:2,version:1,items:[{sku:'AO-1',quantity:2}],label:{strategy:'api',status:'ready',trackingCode:'BX-'+id},...extra});
const line=(id,scanned,status='included')=>({orderId:id,companyId:7,externalRef:'R'+id,recipientName:'Ana',addressLine:'Calle 1',commune:'Maipú',status,packages:scanned.map((sc,i)=>({id:id*10+i,code:`NX-${id}-${i+1}`,scanned:sc}))});
const sockets=[];class FakeWS{constructor(){sockets.push(this);setTimeout(()=>this.onopen?.(),0)}close(){}}
const settle=async()=>{for(let i=0;i<8;i++)await act(async()=>{await new Promise(r=>setTimeout(r,20))})};
const click=async e=>{expect(e,'no encontrado').toBeTruthy();await act(async()=>e.click());await settle()};
const btn=(t,r=document.body)=>[...r.querySelectorAll('button')].find(b=>b.textContent.trim()===t);
const setValue=async(el,v)=>{const proto=el.tagName==='TEXTAREA'?HTMLTextAreaElement:HTMLInputElement;await act(async()=>{Object.getOwnPropertyDescriptor(proto.prototype,'value').set.call(el,v);el.dispatchEvent(new Event('input',{bubbles:true}))})};
const submit=async f=>{await act(async()=>f.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true})));await settle()};
const dialog=()=>[...document.querySelectorAll('[role="dialog"]')].at(-1);
const card=ref=>[...document.querySelectorAll('.manifest-card')].find(c=>c.getAttribute('aria-label')===`Orden ${ref}`);
describe('Retiro Fase 3: manifiesto en pantalla y entrega',()=>{
  let root,host;
  const mount=async()=>{const {DispatchView}=await import('@/features/dispatch');const {Toaster}=await import('@/components/Toast');await act(async()=>root.render(<><DispatchView/><Toaster/></>));await settle()};
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;globalThis.WebSocket=FakeWS;sockets.length=0;sessionStorage.clear();h.session.user.permissions=['despacho'];
    Object.assign(s,{calls:[],fail:null,closedList:[],orders:[order(1),order(2),order(3,{packagesCount:1})],
      open:{id:40,courier:'blueexpress',status:'open',lines:[line(1,[true,true]),line(2,[true,false]),line(3,[false]),line(4,[false],'removed')]},
      closeResult:{delivered:[1],pending:[2,3]}});
    globalThis.__nav={params:new URLSearchParams('courier=blueexpress'),calls:[]};host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});

  it('encabezado, acordeones con tarjetas horizontales y aviso naranjo de faltantes',async()=>{
    await mount();
    const panel=host.querySelector('.manifest-panel');
    expect(panel.querySelector('h2').textContent).toBe('Manifiesto #40 · BlueExpress');
    const facts=Object.fromEntries([...panel.querySelectorAll('.manifest-facts>div')].map(d=>[d.querySelector('dt').textContent,d.querySelector('dd').textContent]));
    expect(facts).toMatchObject({Courier:'BlueExpress',Estado:'Abierto','Órdenes':'3',Bultos:'5','Escaneados':'3/5'});
    expect([...panel.querySelectorAll('summary')].map(s=>s.textContent)).toEqual(['Órdenes (3)','Bultos (5)']);
    expect(panel.querySelectorAll('.manifest-strip').length).toBe(2);
    expect(card('R1').className).toContain('is-complete');expect(card('R1').textContent).toContain('BX-1');
    expect(card('R2').querySelector('.manifest-warning').textContent).toContain('Faltan 1 bulto: NX-2-2');
    const bundle=[...panel.querySelectorAll('.manifest-card')].find(c=>c.getAttribute('aria-label')==='Bulto NX-2-2');
    expect(bundle.textContent).toContain('#R2');expect(bundle.textContent).toContain('Mochila Wilson · SKU AO-1 · 2');
    expect(panel.textContent).toContain('no se imprime');
  });
  it('con un manifiesto abierto no se genera otro',async()=>{
    await mount();const gen=btn('Generar manifiesto',host);expect(gen.disabled).toBe(true);expect(host.textContent).toContain('antes de generar otro');
  });
  it('«Despachar igual»: deshabilitado sin bultos escaneados; pide quién retira y por qué',async()=>{
    await mount();
    const none=btn('Despachar igual',card('R3'));expect(none.disabled).toBe(true);expect(none.parentElement.title).toContain('Escanea al menos un bulto');
    expect(btn('Despachar igual',card('R1'))).toBeUndefined();
    await click(btn('Despachar igual',card('R2')));
    const d=dialog();for(const x of ['#R2','NX-2-2','Jefe Demo'])expect(d.textContent).toContain(x);
    await submit(d.querySelector('form'));expect(d.textContent).toContain('Indica quién retira.');expect(d.textContent).toContain('Indica por qué sale');
    const [picker,reason]=[d.querySelector('input'),d.querySelector('textarea')];
    await setValue(picker,'Juan Pérez');await setValue(reason,'Bulto 2 dañado, se reenvía');await submit(d.querySelector('form'));
    expect(card('R2').querySelector('.manifest-authorized').textContent).toContain('Autorizada sin NX-2-2 · retira Juan Pérez');
    expect(JSON.parse(sessionStorage.getItem('nexus:dispatch:manifest:40'))[0]).toMatchObject({orderId:2,pickerName:'Juan Pérez',authorizedBy:'Jefe Demo'});
    await click(btn('Quitar autorización',card('R2')));expect(card('R2').querySelector('.manifest-authorized')).toBeNull();
  });
  it('Confirmar entrega: notas con empresa y autorizaciones, y resumen con motivos',async()=>{
    await mount();
    await click(btn('Despachar igual',card('R2')));let d=dialog();await setValue(d.querySelector('input'),'Juan Pérez');await setValue(d.querySelector('textarea'),'Bulto dañado');await submit(d.querySelector('form'));
    await click(btn('Confirmar entrega',host));d=dialog();
    expect(d.querySelector('input').value).toBe('Juan Pérez');
    await setValue(d.querySelector('input'),'');await submit(d.querySelector('form'));expect(d.textContent).toContain('Indica quién retira.');expect(s.calls.some(c=>/close$/.test(c.path))).toBe(false);
    const inputs=[...d.querySelectorAll('input')];await setValue(inputs[0],'Juan Pérez');await setValue(inputs[1],'12.345.678-9');await setValue(inputs[2],'ab12cd');await setValue(inputs[3],'Transportes Sur');await setValue(d.querySelector('textarea'),'Retiro de la tarde');
    await submit(d.querySelector('form'));
    const body=s.calls.find(c=>/close$/.test(c.path)).body;
    expect(body).toMatchObject({pickerName:'Juan Pérez',pickerId:'12.345.678-9',vehiclePlate:'ab12cd'});
    const notes=body.notes.split('\n');expect(notes[0]).toBe('Empresa del transporte: Transportes Sur');expect(notes[1]).toBe('Retiro de la tarde');
    expect(notes[2]).toMatch(/^Autorizado sin bulto · #R2 · NX-2-2 · Retira: Juan Pérez · Autoriza: Jefe Demo · .+ · Motivo: Bulto dañado$/);
    const summary=dialog();expect(summary.querySelector('h2,.modal-title')?.textContent??summary.textContent).toContain('Manifiesto #40 cerrado');
    const blocks=Object.fromEntries([...summary.querySelectorAll('.delivery-block')].map(b=>[b.querySelector('h3').textContent,b.querySelector('ul')?.textContent??'']));
    expect(blocks['Entregadas al courier (1)']).toContain('#R1');
    expect(blocks['Para el siguiente retiro (2)']).toContain('#R2 · Despacho autorizado y registrado');expect(blocks['Para el siguiente retiro (2)']).toContain('#R3 · Faltan bultos por escanear');
    expect(blocks['Incidencias (1)']).toContain('#R4 · Problema en el retiro: Caja rota');
    expect(sessionStorage.getItem('nexus:dispatch:manifest:40')).toBeNull();
    expect([...document.querySelectorAll('.toast-warning')].map(t=>t.textContent).join()).toContain('1 orden entregada, 2 para el siguiente retiro');
  });
  it('Error al cerrar: se muestra en el formulario y el manifiesto sigue abierto',async()=>{
    s.fail=new ApiError('El manifiesto ya está cerrado.',409,'conflict');
    await mount();await click(btn('Confirmar entrega',host));const d=dialog();await setValue(d.querySelector('input'),'Ana');await submit(d.querySelector('form'));
    expect(d.textContent).toContain('ya está cerrado');expect(host.querySelector('.manifest-panel')).toBeTruthy();
  });
  it('Sin el permiso Despacho: el manifiesto se ve, sin acciones',async()=>{
    h.session.user.permissions=[];await mount();
    expect(btn('Despachar igual',card('R2'))).toBeUndefined();expect(btn('Confirmar entrega',host).disabled).toBe(true);
  });
  it('Manifiestos cerrados: lectura con quien retiró, empresa y notas',async()=>{
    s.closedList=[{id:39,courier:'blueexpress',status:'closed',closedAt:'2026-11-06T12:00:00Z',closedBy:'user:2 Jefe Demo',pickerName:'Pedro Soto',pickerId:'1-9',vehiclePlate:'XY99',notes:'Empresa del transporte: Transportes Sur\nAutorizado sin bulto · #R2 · NX-2-2',lines:[line(1,[true,true],'delivered'),line(2,[true,false],'pending_packages')]}];
    globalThis.__nav.params=new URLSearchParams('view=closed');await mount();
    expect([...host.querySelectorAll('.view-tabs a')].map(a=>a.textContent)).toEqual(['Entrega de órdenes','Manifiestos cerrados']);
    const m=host.querySelector('.closed-manifest');expect(m.querySelector('summary').textContent).toContain('Manifiesto #39 · BlueExpress');expect(m.querySelector('summary').textContent).toContain('1 de 2 órdenes entregadas');
    for(const x of ['Pedro Soto (1-9)','XY99','Transportes Sur','Autorizado sin bulto · #R2'])expect(m.textContent).toContain(x);
    expect(m.querySelector('.closed-notes').textContent).not.toContain('Empresa del transporte');
    expect([...m.querySelectorAll('.manifest-accordions details')].every(x=>!x.open)).toBe(true);
    expect(btn('Despachar igual',m)).toBeUndefined();expect(btn('Confirmar entrega',host)).toBeUndefined();
  });
});
