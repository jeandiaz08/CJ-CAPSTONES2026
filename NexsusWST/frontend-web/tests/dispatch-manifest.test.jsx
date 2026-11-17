import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act} from 'react';
const h=vi.hoisted(()=>{
  class ApiError extends Error{constructor(m,s,c,f=[],r){super(m);this.status=s;this.code=c;this.fields=f;this.ref=r}}
  return {ApiError,session:{user:{id:2,role:'jefe_operaciones',permissions:['despacho'],name:'Jefe Demo',allCompanies:true,companies:[]}},s:{}};
});
const {s,ApiError}=h;
vi.mock('@/state/auth',()=>({ROLE_LABELS:{},useAuth:()=>({session:h.session,companyId:null,logout:()=>{}})}));
vi.mock('@/services/orderReference',()=>({productsForCompany:async()=>[{sku:'AO-1',description:'Mouse'},{sku:'AO-2',description:'Teclado'}]}));
vi.mock('@/services/api',()=>({ApiError,apiErrorMessage:e=>e.message,downloadFile:async()=>{},printPdf:async()=>{},saveBlob:()=>{},wsUrl:p=>'ws://x'+p,withCompany:p=>p,api:async(_s,path,init)=>{
  if(path==='/v1/ws-ticket')return {ticket:'t'};
  const method=init?.method??'GET';const body=init?.body?JSON.parse(init.body):undefined;s.calls.push({path,method,body});await new Promise(r=>setTimeout(r,2));
  if(method==='POST'&&/\/close$/.test(path)){if(s.fail){const f=s.fail;s.fail=null;throw f}s.open=null;s.orders=s.orders.filter(o=>!s.closeResult.delivered.includes(o.id));return s.closeResult}
  if(path==='/v1/dispatch/couriers')return {data:[{courier:'blueexpress',orders:3,packages:5}]};
  if(path==='/v1/dispatch/manifests?status=open')return {data:s.open?[s.open]:[]};
  if(path.startsWith('/v1/orders?'))return {data:s.orders,total:s.orders.length};
  const m=path.match(/^\/v1\/orders\/(\d+)$/);if(m)return {order:{incidentLabel:'Problema en el retiro',note:'Caja rota'},events:[],documents:[],packages:[]};
  if(path.includes('clients'))return {data:[{id:7,name:'Andes Outdoor SpA (demo)',active:true}]};
  if(path.includes('catalogs/couriers'))return {data:[{code:'blueexpress',name:'BlueExpress'}]};
  return {data:[]};
}}));
const order=(id,extra={})=>({id,externalRef:'R'+id,companyId:7,courier:'blueexpress',status:'ready_pickup',commune:'Maipú',packagesCount:2,version:1,items:[{sku:'AO-1',quantity:3},{sku:'AO-2',quantity:1}],label:{strategy:'api',status:'ready',trackingCode:'BX-'+id},...extra});
const line=(id,scanned,status='included')=>({orderId:id,companyId:7,externalRef:'R'+id,recipientName:'Ana',addressLine:'Calle 1',commune:'Maipú',status,packages:scanned.map((sc,i)=>({id:id*10+i,code:`NX-${id}-${i+1}`,scanned:sc}))});
const sockets=[];class FakeWS{constructor(){sockets.push(this);setTimeout(()=>this.onopen?.(),0)}close(){}}
const settle=async()=>{for(let i=0;i<8;i++)await act(async()=>{await new Promise(r=>setTimeout(r,20))})};
const click=async e=>{expect(e,'no encontrado').toBeTruthy();await act(async()=>e.click());await settle()};
const btn=(t,r=document.body)=>[...r.querySelectorAll('button')].find(b=>b.textContent.trim()===t);
const setValue=async(el,v)=>{const proto=el.tagName==='TEXTAREA'?HTMLTextAreaElement:HTMLInputElement;await act(async()=>{Object.getOwnPropertyDescriptor(proto.prototype,'value').set.call(el,v);el.dispatchEvent(new Event('input',{bubbles:true}))})};
const submit=async f=>{await act(async()=>f.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true})));await settle()};
const dialogs=()=>[...document.querySelectorAll('[role="dialog"]')];
const manifest=()=>document.querySelector('.manifest-modal');
const table=label=>manifest().querySelector(`[aria-label="${label}"]`);
const orderRow=ref=>[...table('Órdenes del manifiesto').querySelectorAll('tbody tr')].find(r=>r.textContent.includes('#'+ref));
describe('Retiro: manifiesto en un modal y entrega',()=>{
  let root,host;
  const mount=async()=>{const {DispatchView}=await import('@/features/dispatch');const {Toaster}=await import('@/components/Toast');await act(async()=>root.render(<><DispatchView/><Toaster/></>));await settle()};
  const openModal=async()=>click(btn('Ver manifiesto abierto #40',host));
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;globalThis.WebSocket=FakeWS;sockets.length=0;sessionStorage.clear();h.session.user.permissions=['despacho'];
    Object.assign(s,{calls:[],fail:null,orders:[order(1),order(2),order(3,{packagesCount:1})],
      open:{id:40,courier:'blueexpress',status:'open',lines:[line(1,[true,true]),line(2,[true,false]),line(3,[false]),line(4,[false],'removed')]},
      closeResult:{delivered:[1,2],pending:[3],incomplete:[2]}});
    globalThis.__nav={params:new URLSearchParams('courier=blueexpress'),calls:[]};host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});

  it('solo la pestaña «Entrega de órdenes»; el manifiesto abierto se abre desde el conteo y no se genera otro',async()=>{
    await mount();
    expect([...host.querySelectorAll('.view-tabs a')].map(a=>a.textContent)).toEqual(['Entrega de órdenes']);
    expect(host.querySelector('.manifest-modal')).toBeNull();expect(btn('Generar manifiesto',host).disabled).toBe(true);
    await openModal();expect(dialogs().length).toBe(1);expect(manifest()).toBeTruthy();
  });
  it('modal: encabezado, «Órdenes» abierto con tabla compacta y «Bultos» cerrado',async()=>{
    await mount();await openModal();
    expect(manifest().textContent).toContain('Manifiesto #40 · BlueExpress');
    const facts=Object.fromEntries([...manifest().querySelectorAll('.manifest-facts>div')].map(d=>[d.querySelector('dt').textContent,d.querySelector('dd').textContent]));
    expect(facts).toMatchObject({Courier:'BlueExpress',Estado:'Abierto','Órdenes':'3',Bultos:'5','Escaneados':'3/5'});
    const acc=[...manifest().querySelectorAll('.manifest-accordion')];
    expect(acc.map(a=>a.querySelector('summary').textContent)).toEqual(['Órdenes (3)','Bultos (5)']);expect(acc.map(a=>a.open)).toEqual([true,false]);
    expect(manifest().querySelectorAll('.manifest-card').length).toBe(0);
    expect([...table('Órdenes del manifiesto').querySelectorAll('th')].map(t=>t.textContent)).toEqual(['N° orden','Cliente','Comuna','Bultos','Código de etiqueta','Acción']);
    expect([...orderRow('R2').querySelectorAll('td')].slice(0,5).map(t=>t.textContent)).toEqual(['#R2','Andes Outdoor SpA (demo)','Maipú','1/2','BX-2']);
    expect(orderRow('R2').className).toContain('is-incomplete');expect(orderRow('R2').querySelector('.manifest-count').className).toContain('is-partial');
    expect(orderRow('R1').querySelector('.manifest-count').className).toContain('is-complete');
    expect(orderRow('R2').querySelector('.cell-truncate').title).toBe('Andes Outdoor SpA (demo)');
    expect([...table('Bultos del manifiesto').querySelectorAll('th')].map(t=>t.textContent)).toEqual(['Bulto','Estado','Código de etiqueta','Orden','Productos']);
    const b=[...table('Bultos del manifiesto').querySelectorAll('tbody tr')].find(r=>r.textContent.startsWith('NX-2-2'));
    expect([...b.querySelectorAll('td')].slice(0,4).map(t=>t.textContent)).toEqual(['NX-2-2','Sin escanear','BX-2','#R2']);
    // Productos completos, uno por línea y sin elipsis.
    const cell=b.querySelector('td.is-products');expect(cell.querySelector('.cell-truncate')).toBeNull();
    expect([...cell.querySelectorAll('.bundle-products li')].map(l=>l.textContent)).toEqual(['Mouse ×3','Teclado ×1']);
  });
  it('«Despachar igual»: solo en incompletas; deshabilitado sin bultos escaneados; pide quién retira y por qué',async()=>{
    await mount();await openModal();
    expect(btn('Despachar igual',orderRow('R1'))).toBeUndefined();
    const none=btn('Despachar igual',orderRow('R3'));expect(none.disabled).toBe(true);expect(none.parentElement.title).toContain('Escanea al menos un bulto');
    await click(btn('Despachar igual',orderRow('R2')));
    const d=dialogs().at(-1);for(const x of ['#R2','NX-2-2','Jefe Demo'])expect(d.textContent).toContain(x);
    await submit(d.querySelector('form'));expect(d.textContent).toContain('Indica quién retira.');
    await setValue(d.querySelector('input'),'Juan Pérez');await setValue(d.querySelector('textarea'),'roto');await submit(d.querySelector('form'));
    expect(d.textContent).toContain('Escribe al menos 5 caracteres.');
    await setValue(d.querySelector('textarea'),'Bulto 2 dañado');await submit(d.querySelector('form'));
    const remove=btn('Quitar autorización',orderRow('R2'));expect(remove.title).toBe('Autorizada sin NX-2-2 · retira Juan Pérez');
    await click(remove);expect(btn('Despachar igual',orderRow('R2'))).toBeTruthy();
  });
  it('«Agregar observación» por orden (opcional) y queda en las notas',async()=>{
    await mount();await openModal();
    await click(btn('Agregar observación',orderRow('R1')));let d=dialogs().at(-1);
    await setValue(d.querySelector('textarea'),'Faltó el cable del mouse');await submit(d.querySelector('form'));
    expect(btn('Editar observación',orderRow('R1')).title).toBe('Faltó el cable del mouse');
    await click(btn('Confirmar entrega',manifest()));d=dialogs().at(-1);await setValue(d.querySelector('input'),'Ana');await submit(d.querySelector('form'));
    const body=s.calls.find(c=>/close$/.test(c.path)).body;
    expect(body.notes).toBe('Observación · #R1 · Faltó el cable del mouse');expect(body.dispatchAnyway).toEqual([]);
  });
  it('Confirmar entrega: «Despachar igual» va en dispatchAnyway, sale entregada y el resumen muestra lo incompleto',async()=>{
    await mount();await openModal();
    await click(btn('Despachar igual',orderRow('R2')));let d=dialogs().at(-1);await setValue(d.querySelector('input'),'Juan Pérez');await setValue(d.querySelector('textarea'),'Bulto dañado');await submit(d.querySelector('form'));
    await click(btn('Confirmar entrega',manifest()));d=dialogs().at(-1);
    expect(d.querySelector('input').value).toBe('Juan Pérez');
    // Diseño de la imagen: «Quien retira» con Nombre conductor, RUT y Patente; luego el transporte y el aviso.
    expect([...d.querySelectorAll('.delivery-form h3')].map(x=>x.textContent)).toEqual(['Quien retira','Transporte']);
    expect([...d.querySelectorAll('label')].map(x=>x.textContent)).toEqual(['Nombre conductor *','RUT','Patente','Empresa del transporte','Notas']);
    expect([...d.querySelectorAll('.delivery-pair label')].map(x=>x.textContent)).toEqual(['RUT','Patente']);
    expect(d.querySelector('.delivery-hint').textContent).toBe('Fecha y hora se registran solas, después de cerrar cualquier cambio genera incidencia');
    expect([...d.querySelectorAll('.form-modal-foot button')].map(x=>x.textContent)).toEqual(['Confirmar entrega','Cancelar']);
    const inputs=[...d.querySelectorAll('input')];await setValue(inputs[3],'Transportes Sur');await setValue(d.querySelector('textarea'),'Retiro de la tarde');
    await submit(d.querySelector('form'));
    const body=s.calls.find(c=>/close$/.test(c.path)).body;
    expect(body.notes.split('\n')).toEqual(['Empresa del transporte: Transportes Sur','Retiro de la tarde']);
    expect(body.dispatchAnyway).toEqual([{orderId:2,reason:'Bulto dañado'}]);
    const summary=dialogs().at(-1);
    const blocks=Object.fromEntries([...summary.querySelectorAll('.delivery-block')].map(b=>[b.querySelector('h3').textContent,b.querySelector('ul')?.textContent??'']));
    expect(blocks['Entregadas al courier (2)']).toContain('#R1');
    expect(blocks['Entregadas al courier (2)']).toContain('#R2 · Con bultos faltantes (NX-2-2) · autorizó Jefe Demo · motivo: Bulto dañado');
    expect(blocks['Para el siguiente retiro (1)']).toContain('#R3 · Faltan bultos por escanear');
    expect(blocks['Incidencias (1)']).toContain('#R4 · Problema en el retiro: Caja rota');
    expect(document.querySelector('.toast-warning').textContent).toContain('2 órdenes entregadas (1 con bultos faltantes), 1 para el siguiente retiro');
    await click(btn('Cerrar',summary.querySelector('.form-modal-foot')));
    expect(document.querySelector('.manifest-modal')).toBeNull();expect(sessionStorage.getItem('nexus:dispatch:manifest:40')).toBeNull();
    const refs=[...host.querySelectorAll('.dispatch-table tbody tr')].map(r=>r.textContent);
    expect(refs.some(t=>t.includes('#R2'))).toBe(false);expect(refs.some(t=>t.includes('#R3'))).toBe(true);
  });
  it('Autorizada pero el servidor no la entrega: queda para el siguiente retiro con el motivo',async()=>{
    s.closeResult={delivered:[1],pending:[2,3],incomplete:[]};
    await mount();await openModal();
    await click(btn('Despachar igual',orderRow('R2')));let d=dialogs().at(-1);await setValue(d.querySelector('input'),'Juan Pérez');await setValue(d.querySelector('textarea'),'Bulto dañado');await submit(d.querySelector('form'));
    await click(btn('Confirmar entrega',manifest()));d=dialogs().at(-1);await submit(d.querySelector('form'));
    const pending=[...dialogs().at(-1).querySelectorAll('.delivery-block')].find(b=>b.querySelector('h3').textContent==='Para el siguiente retiro (2)');
    expect(pending.textContent).toContain('#R2 · Despacho autorizado, pero el servidor no pudo entregarla');
  });
  it('Error al cerrar: se muestra en el formulario y el modal sigue abierto',async()=>{
    s.fail=new ApiError('El manifiesto ya está cerrado.',409,'conflict');
    await mount();await openModal();await click(btn('Confirmar entrega',manifest()));const d=dialogs().at(-1);await setValue(d.querySelector('input'),'Ana');await submit(d.querySelector('form'));
    expect(d.textContent).toContain('ya está cerrado');expect(manifest()).toBeTruthy();
  });
  it('Sin el permiso Despacho: el manifiesto se ve, sin acciones',async()=>{
    h.session.user.permissions=[];await mount();await openModal();
    expect(btn('Despachar igual',manifest())).toBeUndefined();expect(btn('Agregar observación',manifest())).toBeUndefined();expect(btn('Confirmar entrega',manifest()).disabled).toBe(true);
  });
});
