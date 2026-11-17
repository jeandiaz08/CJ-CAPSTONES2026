import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest';
import {createRoot} from 'react-dom/client';
import {act,useState} from 'react';
const h=vi.hoisted(()=>({events:[]}));
vi.mock('@/state/auth',()=>({ROLE_LABELS:{},useAuth:()=>({session:{user:{id:2,role:'jefe_operaciones',permissions:[],name:'J',allCompanies:true,companies:[]}},companyId:null,logout:()=>{}})}));
vi.mock('@/services/orderReference',()=>({productsForCompany:async()=>[]}));
vi.mock('@/services/api',()=>({ApiError:class extends Error{},apiErrorMessage:e=>e.message,downloadFile:async()=>{},wsUrl:p=>'ws://x'+p,withCompany:p=>p,
  api:async(_s,path)=>{
    if(/^\/v1\/orders\/\d+/.test(path))return {order:{id:15,externalRef:'48257769215',companyId:7,status:'incident',statusLabel:'Incidencia',items:[]},events:h.events,documents:[],packages:[]};
    return {data:[]};
  }}));
const settle=async()=>{for(let i=0;i<6;i++)await act(async()=>{await new Promise(r=>setTimeout(r,10))})};
const key=async k=>act(async()=>{document.dispatchEvent(new KeyboardEvent('keydown',{key:k,bubbles:true}))});
const ev=(id,at,extra)=>({id,type:'status_change',event:'x',fromStatus:'',toStatus:'',actor:'user:2 Jefe Demo',occurredAt:at,...extra});

describe('Bloqueo de scroll de los modales',()=>{
  let root,host;
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;document.body.style.overflow='';host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});

  // Dos modales que el padre puede abrir y cerrar en cualquier orden.
  let api,Modal;
  function Two(){const [a,setA]=useState(false),[b,setB]=useState(false);api={setA,setB};
    return <>{a&&<Modal title="A" onClose={()=>setA(false)}><p>a</p></Modal>}{b&&<Modal title="B" onClose={()=>setB(false)}><p>b</p></Modal>}</>}
  beforeEach(async()=>{({Modal}=await import('@/components/Modal'))});
  const mount=async()=>{await act(async()=>root.render(<Two/>))};
  const set=async(k,v)=>act(async()=>api[k](v));

  it('Escape, clic afuera y la X devuelven el scroll',async()=>{
    await mount();
    await set('setA',true);expect(document.body.style.overflow).toBe('hidden');
    await key('Escape');expect(document.querySelector('.modal')).toBeNull();expect(document.body.style.overflow).toBe('');
    await set('setA',true);await act(async()=>document.querySelector('.modal-backdrop').click());expect(document.body.style.overflow).toBe('');
    await set('setA',true);await act(async()=>document.querySelector('.modal [aria-label="Cerrar"]').click());expect(document.body.style.overflow).toBe('');
  });
  it('Dos modales apilados: el scroll vuelve solo al cerrar ambos, en cualquier orden',async()=>{
    await mount();
    await set('setA',true);await set('setB',true);
    await set('setA',false);expect(document.body.style.overflow).toBe('hidden'); // el de abajo se cierra primero
    await set('setB',false);expect(document.body.style.overflow).toBe('');
    await set('setA',true);await set('setB',true);
    await key('Escape');expect(document.querySelectorAll('.modal').length).toBe(1);expect(document.body.style.overflow).toBe('hidden');
    await key('Escape');expect(document.body.style.overflow).toBe('');
  });
  it('Navegación (desmontar con modales abiertos) y el menú móvil cerrándose antes devuelven el scroll',async()=>{
    const {lockScroll}=await import('@/hooks/scrollLock');
    await mount();
    const menu=lockScroll();                    // menú móvil abierto
    await set('setA',true);menu();              // navega: el menú se cierra con el modal abierto
    expect(document.body.style.overflow).toBe('hidden');
    await act(async()=>root.unmount());root=createRoot(host); // la página cambia
    expect(document.body.style.overflow).toBe('');
    menu();expect(document.body.style.overflow).toBe(''); // liberar dos veces no descuadra el contador
    const again=lockScroll();expect(document.body.style.overflow).toBe('hidden');again();expect(document.body.style.overflow).toBe('');
  });
});

describe('Filtros e historial',()=>{
  let root,host;
  beforeEach(()=>{globalThis.IS_REACT_ACT_ENVIRONMENT=true;host=document.createElement('div');document.body.append(host);root=createRoot(host)});
  afterEach(async()=>{await act(async()=>root.unmount());host.remove();document.body.innerHTML=''});

  it('El modal de Filtros se abre centrado, como los demás',async()=>{
    const {FiltersModal}=await import('@/components/FiltersModal');
    await act(async()=>root.render(<FiltersModal title="Filtros" onClose={()=>{}} footer={<button>Aplicar</button>}><p>campos</p></FiltersModal>));
    const panel=document.querySelector('.modal');
    expect(panel.className).toContain('form-modal');expect(panel.className).toContain('filters-modal');
    expect(document.querySelector('.modal-backdrop').className.trim()).toBe('modal-backdrop');
    expect(document.querySelector('.side-drawer')).toBeNull();
  });
  it('«Ver todo» muestra todos los eventos de la orden en horizontal, del primero al último',async()=>{
    h.events=[ev(3,'2026-11-05T10:00:00-03:00',{fromStatus:'ready_pickup',toStatus:'incident',reason:'Caja rota'}),
      ev(1,'2026-11-01T09:00:00-03:00',{type:'created',event:'create',toStatus:'validating',actor:'Sistema'}),
      ev(2,'2026-11-02T09:00:00-03:00',{type:'note',event:'prioritize'}),
      ev(4,'2026-11-03T09:00:00-03:00',{fromStatus:'validating',toStatus:'ready_pickup'})];
    const {OrderDetail}=await import('@/features/orders/components/OrderDetail');
    const labels={validating:'En validación',ready_pickup:'Listo para retiro',incident:'Incidencia'};
    await act(async()=>root.render(<OrderDetail id={15} companyId={null} role="jefe_operaciones" revision={0} labels={labels}/>));await settle();
    expect(document.querySelector('.event-timeline')).toBeNull();
    await act(async()=>[...host.querySelectorAll('button')].find(b=>b.textContent==='Ver todo').click());
    const modal=document.querySelector('.modal.history-modal');expect(modal).toBeTruthy();
    const list=modal.querySelector('ol.event-timeline.is-horizontal');expect(list).toBeTruthy();
    expect([...list.querySelectorAll('li strong')].map(s=>s.textContent)).toEqual(['Creada · En validación','Priorizada (primero en la cola)','Listo para retiro','Incidencia']);
    expect(list.querySelector('li:last-child').className).toContain('is-danger');
    expect(list.querySelector('li:last-child').textContent).toContain('Caja rota');
    await key('Escape');expect(document.querySelector('.history-modal')).toBeNull();expect(document.body.style.overflow).toBe('');
  });
});
