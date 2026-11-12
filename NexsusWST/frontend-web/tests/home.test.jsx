import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act} from 'react';
import {dateInDays,dueLabel,homeCards,quickLinks} from '@/features/home/homeConfig';

const h=vi.hoisted(()=>({session:null,calls:[],notes:[],nextId:10,summary:{},integrations:[],pending:0,toasts:[]}));
vi.mock('@/state/auth',()=>({useAuth:()=>({session:h.session,companyId:h.session?.user.companies?.[0]??null})}));
vi.mock('@/components/Toast',()=>({showToast:(m,t)=>h.toasts.push([m,t??'success'])}));
vi.mock('@/components/NotificationBell',()=>({NotificationBell:()=>null}));
vi.mock('@/hooks/useOrdersSocket',()=>({useOrdersSocket:()=>true}));
vi.mock('@/services/api',()=>({ApiError:class extends Error{},apiErrorMessage:e=>e.message,withCompany:(p,c)=>c==null?p:p+'?company_id='+c,
  api:async(service,path,init)=>{
    const method=init?.method??'GET';h.calls.push([service,method,path,init?.body?JSON.parse(init.body):undefined]);
    if(path.startsWith('/v1/orders/summary'))return {data:h.summary};
    if(path==='/v1/integrations')return {data:h.integrations};
    if(path.startsWith('/v1/profile-requests'))return {data:[],total:0,counts:{pending:h.pending,approved:0,rejected:0}};
    if(path==='/v1/me/notes'&&method==='GET')return {data:h.notes};
    if(path==='/v1/me/notes'&&method==='POST'){const b=JSON.parse(init.body);const n={id:h.nextId++,body:b.body,dueDate:b.dueDate,done:false};h.notes=[...h.notes,n];return n}
    const id=Number(path.split('/').pop());
    if(method==='PATCH'){h.notes=h.notes.map(n=>n.id===id?{...n,...JSON.parse(init.body)}:n);return h.notes.find(n=>n.id===id)}
    if(method==='DELETE'){h.notes=h.notes.filter(n=>n.id!==id);return undefined}
    throw new Error('sin simular: '+path);
  }}));

const u=(role,permissions=[])=>({role,permissions});
const titles=user=>homeCards(user).map(c=>c.title);
const labels=user=>quickLinks(user).map(l=>l.label);
const settle=async()=>{for(let i=0;i<6;i++)await act(async()=>{await new Promise(r=>setTimeout(r,10))})};
const click=async e=>{expect(e,'no encontrado').toBeTruthy();await act(async()=>e.click());await settle()};
const type=async(input,value)=>{await act(async()=>{Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,value);input.dispatchEvent(new Event('input',{bubbles:true}))});await settle()};
const button=(host,text)=>[...host.querySelectorAll('button')].find(b=>b.textContent.trim()===text);

describe('Home · tarjetas y accesos por perfil',()=>{
  it('Administrador: la tercera tarjeta depende de «Gestionar perfiles»',()=>{
    expect(titles(u('admin',['gestionar_perfiles']))).toEqual(['Órdenes pendientes','Integraciones activas','Solicitudes de perfiles pendientes']);
    expect(titles(u('admin'))).toEqual(['Órdenes pendientes','Integraciones activas','Incidencias abiertas']);
    expect(labels(u('admin'))).toEqual(['Todas las órdenes','Órdenes manuales','Solicitudes','Registro de acciones','Integraciones']);
  });
  it('Jefe de Operaciones: estados reales de la bandeja y Retiro solo con Despacho',()=>{
    const cards=homeCards(u('jefe_operaciones'));
    expect(cards.map(c=>c.title)).toEqual(['Órdenes por validar','Órdenes con incidencia','Listas para retiro']);
    expect(cards.map(c=>c.source.statuses)).toEqual([['pending','validating'],['incident'],['ready_pickup']]);
    expect(cards[0].href).toBe('/ordenes/todas/listado?status=pending,validating');
    expect(labels(u('jefe_operaciones',['despacho']))).toEqual(['Todas las órdenes','Órdenes manuales','Retiro y entrega','Registro histórico']);
    expect(labels(u('jefe_operaciones'))).toEqual(['Todas las órdenes','Órdenes manuales','Registro histórico']);
  });
  it('Empaque: «Listas para retiro» y Retiro solo con Despacho; nada de Integraciones ni órdenes',()=>{
    expect(titles(u('empaque',['despacho']))).toEqual(['Por empacar','Listas para retiro']);
    expect(titles(u('empaque'))).toEqual(['Por empacar']);
    expect(labels(u('empaque',['despacho']))).toEqual(['Empaque','Retiro y entrega']);
    expect(labels(u('empaque'))).toEqual(['Empaque']);
    expect(homeCards(u('empaque',['despacho'])).map(c=>c.href)).toEqual(['/ordenes/empaque','/ordenes/retiro']);
  });
  it('Cliente: sus tarjetas y accesos; el texto de solicitudes habla de su empresa',()=>{
    expect(titles(u('cliente'))).toEqual(['Mis órdenes en curso','Con incidencia','Solicitudes pendientes de tu empresa']);
    expect(homeCards(u('cliente'))[0].source.statuses).not.toEqual(expect.arrayContaining(['handed_over','voided','cancelled']));
    expect(labels(u('cliente'))).toEqual(['Mis órdenes','Perfiles empresa','Mis solicitudes','Productos']);
  });
  it('Analista: sin tarjetas ni accesos',()=>{expect(titles(u('analista'))).toEqual([]);expect(labels(u('analista'))).toEqual([])});
  it('Fechas: Hoy, Mañana, Ayer, En N días',()=>{
    const today=new Date(2026,10,11,23,30);
    expect(dateInDays(0,today)).toBe('2026-11-11');expect(dateInDays(3,today)).toBe('2026-11-14');expect(dateInDays(1,new Date(2026,10,30))).toBe('2026-12-01');
    expect([null,'2026-11-11','2026-11-12','2026-11-10','2026-11-14','2026-11-08'].map(d=>dueLabel(d,today))).toEqual(['','Hoy','Mañana','Ayer','En 3 días','Hace 3 días']);
  });
});

describe('Home · pantalla',()=>{
  let root,host;
  const mount=async()=>{const {HomeView}=await import('@/features/home');await act(async()=>root.render(<HomeView/>));await settle()};
  const session=(role,permissions=[],extra={})=>({user:{id:1,name:'Ana Pérez',role,permissions,companies:[],allCompanies:true,...extra}});
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;h.calls.length=0;h.toasts.length=0;h.nextId=10;h.pending=4;
    h.summary={pending:3,validating:2,picked:4,ready_pickup:5,incident:6,handed_over:9};
    h.integrations=[{id:1,active:true},{id:2,active:true},{id:3,active:false}];
    h.notes=[{id:1,body:'Revisar stock',dueDate:dateInDays(0),done:false},{id:2,body:'Llamar courier',dueDate:dateInDays(-1),done:true}];
    host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});
  const card=key=>host.querySelector(`[data-card="${key}"]`);

  it('Administrador con «Gestionar perfiles»: saludo, números y enlaces filtrados',async()=>{
    h.session=session('admin',['gestionar_perfiles']);await mount();
    expect(host.querySelector('.home-greeting').textContent).toContain('Bienvenido a NexusWST');
    expect(host.querySelector('.home-greeting h2').textContent).toContain('Hola, Ana');
    expect(host.querySelector('.home-greeting').textContent).toContain('Aquí tienes un resumen de tu operación de hoy');
    expect(card('pending').querySelector('strong').textContent).toBe('3');
    expect(card('pending').getAttribute('href')).toBe('/ordenes/todas/listado?status=pending');
    expect(card('integrations').querySelector('strong').textContent).toBe('2/3');
    expect(card('requests').querySelector('strong').textContent).toBe('4');
    expect(card('requests').getAttribute('href')).toBe('/clientes/administracion?tab=solicitudes');
    expect([...host.querySelectorAll('.home-card')].map(c=>c.className.split(' ')[1])).toEqual(['home-orange','home-blue','home-green']);
    expect(host.textContent).toContain('Acceso rápido');expect(host.textContent).toContain('Llega más rápido donde lo necesites');
  });
  it('Empaque sin Despacho: una tarjeta, un acceso y no consulta integraciones ni solicitudes',async()=>{
    h.session=session('empaque');await mount();
    expect([...host.querySelectorAll('.home-card')].map(c=>c.dataset.card)).toEqual(['to-pack']);
    expect(card('to-pack').querySelector('strong').textContent).toBe('4');
    expect([...host.querySelectorAll('.home-quick a')].map(a=>a.textContent)).toEqual(['Empaque']);
    expect(h.calls.some(c=>c[2]==='/v1/integrations'||c[2].startsWith('/v1/profile-requests'))).toBe(false);
  });
  it('Cliente: «en curso» suma sus estados y pide el resumen de su empresa',async()=>{
    h.session=session('cliente',[],{companies:[7],allCompanies:false});await mount();
    expect(card('in-progress').querySelector('strong').textContent).toBe('20');
    expect(card('incident').querySelector('strong').textContent).toBe('6');
    expect(h.calls.some(c=>c[2]==='/v1/orders/summary?company_id=7')).toBe(true);
    expect(card('requests').textContent).toContain('Solicitudes pendientes de tu empresa');
  });
  it('Recordatorios: lista con fecha y tachado; marcar hecha y borrar',async()=>{
    h.session=session('jefe_operaciones');await mount();
    const rows=()=>[...host.querySelectorAll('.reminders-list li')];
    expect(rows().map(r=>[r.querySelector('label span').textContent,r.querySelector('.reminders-due').textContent,r.classList.contains('is-done')])).toEqual([['Revisar stock','Hoy',false],['Llamar courier','Ayer',true]]);
    await click(rows()[0].querySelector('input[type=checkbox]'));
    expect(h.calls).toContainEqual(['core','PATCH','/v1/me/notes/1',{done:true}]);
    expect(rows()[0].classList.contains('is-done')).toBe(true);
    await click(host.querySelector('[aria-label="Borrar nota: Llamar courier"]'));
    expect(h.calls).toContainEqual(['core','DELETE','/v1/me/notes/2',undefined]);
    expect(rows().length).toBe(1);
  });
  it('Nueva nota: contador visible, tope de 140 y fecha opcional',async()=>{
    h.session=session('admin');await mount();
    await click(button(host,'Nueva nota'));
    const input=host.querySelector('#note-body'),count=()=>host.querySelector('#note-count').textContent,save=()=>button(host,'Guardar');
    expect(count()).toBe('0/140');expect(save().disabled).toBe(true);
    await type(input,'x'.repeat(141));
    expect(count()).toContain('141/140');expect(count()).toContain('Máximo 140 caracteres');expect(save().disabled).toBe(true);
    await type(input,'  Pedir cajas  ');
    expect(count()).toBe('11/140');
    await click([...host.querySelectorAll('.reminders-chip input')].find(r=>r.parentElement.textContent==='En 3 días'));
    await click(save());
    expect(h.calls).toContainEqual(['core','POST','/v1/me/notes',{body:'Pedir cajas',dueDate:dateInDays(3)}]);
    expect(h.toasts).toContainEqual(['Nota agregada','success']);
    expect(host.querySelector('#note-body')).toBeNull();
    expect(host.querySelector('.reminders-list').textContent).toContain('Pedir cajas');
  });
  it('Sin notas: estado vacío',async()=>{
    h.session=session('empaque');h.notes=[];await mount();
    expect(host.querySelector('.reminders-empty').textContent).toContain('Aún no tienes notas');
  });
});
