import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act} from 'react';
const h=vi.hoisted(()=>{
  class ApiError extends Error{constructor(m,s,c,f=[],r){super(m);this.status=s;this.code=c;this.fields=f;this.ref=r}}
  return {ApiError,session:{user:{id:2,role:'jefe_operaciones',permissions:['despacho'],name:'J',allCompanies:true,companies:[]}},s:{}};
});
const {s,ApiError}=h;
vi.mock('@/state/auth',()=>({ROLE_LABELS:{},useAuth:()=>({session:h.session,companyId:null,logout:()=>{}})}));
vi.mock('@/services/api',()=>({ApiError,apiErrorMessage:e=>e.message,downloadFile:async()=>{},printPdf:async()=>{},saveBlob:()=>{},wsUrl:p=>'ws://x'+p,withCompany:p=>p,api:async(_s,path,init)=>{
  if(path==='/v1/ws-ticket')return {ticket:'t'};
  const method=init?.method??'GET';const body=init?.body?JSON.parse(init.body):undefined;s.calls.push({path,method,body});await new Promise(r=>setTimeout(r,2));
  if(method==='POST'&&path==='/v1/dispatch/manifests'){const lines=body.orderIds.map(id=>({orderId:id,status:'included',packages:s.pk[id].map((c,i)=>({id:i,code:c,scanned:false}))}));s.open={id:77,courier:body.courier,status:'open',lines};return {manifest:s.open,excluded:s.excluded??[]}}
  if(method==='POST'&&/\/scan$/.test(path)){if(s.failScan)throw new ApiError('no',422);const l=s.open.lines.find(l=>l.packages.some(p=>p.code===body.code));l.packages.find(p=>p.code===body.code).scanned=true;return l}
  if(path==='/v1/dispatch/couriers')return {data:s.couriers};
  if(path.startsWith('/v1/dispatch/manifests'))return {data:s.open?[s.open]:[]};
  if(path.startsWith('/v1/orders?'))return {data:s.orders.filter(o=>o.courier===new URLSearchParams(path.split('?')[1]).get('courier')),total:0};
  const m=path.match(/^\/v1\/orders\/(\d+)$/);if(m)return {order:{},events:[],documents:[],packages:(s.pk[m[1]]||[]).map((c,i)=>({id:i,code:c,manifestId:null}))};
  if(path.includes('clients'))return {data:[{id:7,name:'Empresa Siete',active:true}]};
  if(path.includes('catalogs/couriers'))return {data:[{code:'blueexpress',name:'BlueExpress'},{code:'recibelo',name:'Recíbelo'}]};
  return {data:[]};
}}));
const ready={strategy:'api',status:'ready',trackingCode:'BX-1'};
const mk=(id,extra={})=>({id,externalRef:'R'+id,companyId:7,courier:'blueexpress',status:'ready_pickup',recipientName:'Ana '+id,commune:'Ñuñoa',packagesCount:1,version:1,label:{...ready,trackingCode:'BX-'+id},...extra});
const sockets=[];class FakeWS{constructor(){sockets.push(this);setTimeout(()=>this.onopen?.(),0)}close(){}}
const settle=async()=>{for(let i=0;i<8;i++)await act(async()=>{await new Promise(r=>setTimeout(r,20))})};
const click=async e=>{expect(e,'no encontrado').toBeTruthy();await act(async()=>e.click());await settle()};
const setValue=async(el,v)=>{await act(async()=>{Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(el,v);el.dispatchEvent(new Event('input',{bubbles:true}))})};
describe('Retiro Fase 1',()=>{
  let root,host;
  const mount=async()=>{const {DispatchView}=await import('@/features/dispatch');const {Toaster}=await import('@/components/Toast');await act(async()=>root.render(<><DispatchView/><Toaster/></>));await settle()};
  const rows=()=>[...host.querySelectorAll('.dispatch-table tbody tr')];
  const scan=async c=>{const i=host.querySelector('#dispatch-reader-input');await setValue(i,c);await act(async()=>i.form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true})));await settle()};
  const notice=()=>host.querySelector('.dispatch-reader-notice');
  const tally=()=>[...host.querySelectorAll('.dispatch-tally strong')].map(x=>x.textContent);
  const gen=()=>[...host.querySelectorAll('button')].find(b=>b.textContent.includes('Generar manifiesto'));
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;globalThis.WebSocket=FakeWS;sockets.length=0;sessionStorage.clear();h.session.user.permissions=['despacho'];
    Object.assign(s,{calls:[],open:null,excluded:null,failScan:false,couriers:[{courier:'blueexpress',orders:3,packages:4},{courier:'recibelo',orders:2,packages:2}],
      orders:[mk(1),mk(2,{packagesCount:2}),mk(3,{label:{strategy:'api',status:'pending'}}),mk(4,{courier:'recibelo',label:{strategy:'office',status:'office'}})],
      pk:{1:['NX-1-1'],2:['NX-2-1','NX-2-2'],3:['NX-3-1'],4:['NX-4-1']}});
    globalThis.__nav={params:new URLSearchParams('courier=blueexpress'),calls:[]};host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});

  it('KPI con el total, tarjetas por courier y la seleccionada resaltada',async()=>{
    await mount();
    expect(host.querySelector('.kpi .kpi-label').textContent).toBe('Lista para el retiro');
    const cards=[...host.querySelectorAll('.dispatch-courier')];
    expect(cards.map(c=>c.querySelector('.dispatch-courier-name').textContent+':'+c.querySelector('strong').textContent)).toEqual(['BlueExpress:3','Recíbelo:2']);
    expect(cards[0].getAttribute('aria-pressed')).toBe('true');expect(cards[1].getAttribute('aria-pressed')).toBe('false');
    await click(cards[1]);expect(globalThis.__nav.calls.at(-1)).toEqual(['replace','/ordenes/todas/listado?courier=recibelo']);
  });
  it('Lista: subtítulo, columnas y fila sin etiqueta bloqueada',async()=>{
    await mount();
    expect(host.querySelector('.dispatch-panel-head').textContent).toContain('3 órdenes – 4 bultos esperados');
    expect([...host.querySelectorAll('.dispatch-table th')].map(t=>t.textContent).slice(1,7)).toEqual(['Código','N° orden','Cliente','Destinatario','Comuna','Bultos']);
    const r=rows();expect(r[0].textContent).toContain('BX-1');expect(r[0].textContent).toContain('Empresa Siete');
    expect(r[2].className).toContain('is-blocked');expect(r[2].textContent).toContain('Etiqueta pendiente');expect(r[2].querySelector('input').disabled).toBe(true);
    expect(tally()).toEqual(['4','0']);expect(gen().disabled).toBe(true);
    expect(document.activeElement.id).toBe('dispatch-reader-input');
  });
  it('Lector: marca, selecciona la orden y pinta parcial/completo; ajeno rojo, repetido amarillo',async()=>{
    await mount();
    await scan('nx-2-1');expect(notice().className).toContain('is-ok');
    expect(rows()[1].querySelector('.dispatch-count').className).toContain('is-partial');expect(rows()[1].querySelector('.dispatch-count').textContent).toBe('1/2');
    expect(rows()[1].querySelector('input').checked).toBe(true);expect(tally()).toEqual(['4','1']);
    expect(host.textContent).toContain('Órdenes seleccionadas 1 de 3');expect(gen().disabled).toBe(false);
    await scan('NX-2-1');expect(notice().className).toContain('is-warn');expect(tally()).toEqual(['4','1']);
    await scan('NX-2-2');expect(rows()[1].querySelector('.dispatch-count').className).toContain('is-complete');
    await scan('OTRO-9');expect(notice().className).toContain('is-error');expect(notice().textContent).toContain('no pertenece a la lista de BlueExpress');
    await scan('NX-3-1');expect(notice().className).toContain('is-error');expect(notice().textContent).toContain('etiqueta pendiente');expect(tally()).toEqual(['4','2']);
  });
  it('Lo marcado sobrevive a una recarga, por courier',async()=>{
    await mount();await scan('NX-1-1');
    await act(async()=>root.unmount());root=createRoot(host);await mount();
    expect(tally()).toEqual(['4','1']);expect(rows()[0].querySelector('input').checked).toBe(true);
  });
  it('Solo se selecciona con al menos un bulto escaneado',async()=>{
    await mount();
    expect(rows()[0].querySelector('input').disabled).toBe(true);expect(rows()[0].querySelector('td').title).toBe('Escanea al menos un bulto');
    await scan('NX-1-1');expect(rows()[0].querySelector('input').disabled).toBe(false);expect(rows()[0].querySelector('input').checked).toBe(true);expect(gen().disabled).toBe(false);
    await click(rows()[0].querySelector('input'));expect(rows()[0].querySelector('input').checked).toBe(false);expect(gen().disabled).toBe(true);
    expect(host.textContent).toContain('Órdenes seleccionadas 0 de 3');
  });
  it('Generar manifiesto: solo las seleccionadas con bultos, reenvía lo leído, limpia y abre el modal',async()=>{
    await mount();await scan('NX-2-1');await scan('NX-1-1');
    await click(gen());
    expect(s.calls.find(c=>c.method==='POST'&&c.path==='/v1/dispatch/manifests').body).toEqual({courier:'blueexpress',orderIds:[1,2]});
    expect(s.calls.filter(c=>/\/scan$/.test(c.path)).map(c=>c.body.code)).toEqual(['NX-1-1','NX-2-1']);
    expect(document.querySelector('[role="dialog"]').textContent).toContain('Manifiesto #77');
    expect(sessionStorage.getItem('nexus:dispatch:blueexpress')).toBeNull();
    expect([...document.querySelectorAll('.toast-success')].map(t=>t.textContent)).toContain('Manifiesto #77 generado con 2 órdenes.');
    expect(rows()[1].textContent).toContain('En manifiesto #77');expect(rows()[1].querySelector('.dispatch-count').textContent).toBe('1/2');
    // Ya en el manifiesto, la lectura va directo al servidor.
    await scan('NX-2-2');expect(s.calls.filter(c=>/\/scan$/.test(c.path)).map(c=>c.body.code)).toEqual(['NX-1-1','NX-2-1','NX-2-2']);expect(rows()[1].querySelector('.dispatch-count').textContent).toBe('2/2');
  });
  it('Excluidas por cobertura: aviso',async()=>{
    s.excluded=['#1: El courier blueexpress no tiene cobertura en Ñuñoa (pasó a Incidencia)'];
    await mount();await scan('NX-1-1');await click(gen());
    expect([...document.querySelectorAll('.toast-warning')].map(t=>t.textContent).join()).toContain('no tiene cobertura');
  });
  it('En vivo: una orden que sale de la lista se va y deja de contar',async()=>{
    await mount();await scan('NX-1-1');expect(tally()).toEqual(['4','1']);
    s.orders=s.orders.filter(o=>o.id!==1);
    await act(async()=>sockets.at(-1).onmessage({data:JSON.stringify({type:'order.changed',orderId:1})}));await settle();await settle();
    expect(rows().length).toBe(2);expect(tally()).toEqual(['3','0']);expect(host.textContent).toContain('Órdenes seleccionadas 0 de 2');
  });
  it('Recíbelo («Buscar en oficina») se puede entregar sin etiqueta',async()=>{
    globalThis.__nav.params=new URLSearchParams('courier=recibelo');await mount();
    expect(rows()[0].className).not.toContain('is-blocked');await scan('NX-4-1');expect(notice().className).toContain('is-ok');
  });
  it('Sin el permiso Despacho: solo lectura',async()=>{
    h.session.user.permissions=[];await mount();
    expect(host.querySelector('#dispatch-reader-input').disabled).toBe(true);expect(gen().disabled).toBe(true);
    expect(host.textContent).toContain('Solo lectura');expect(rows()[0].querySelector('input').disabled).toBe(true);
  });
  it('Sin courier elegido: pide elegir uno',async()=>{
    globalThis.__nav.params=new URLSearchParams();await mount();
    expect(host.textContent).toContain('Elige un courier');expect(s.calls.some(c=>c.path.startsWith('/v1/orders?'))).toBe(false);
  });
});
