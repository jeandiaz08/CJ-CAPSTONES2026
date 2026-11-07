import {describe,it,expect} from 'vitest';
import {canAccessPath,visibleMenu} from '@/state/menuAccess';
const u=(role,permissions=[])=>({role,permissions});
const kids=user=>visibleMenu(user).find(e=>e.name==='Órdenes')?.children?.map(c=>c.href);
describe('Retiro en el mapa de acceso',()=>{
  it('el menú apunta a /ordenes/retiro solo con despacho (Admin siempre, en lectura)',()=>{
    expect(kids(u('jefe_operaciones',['despacho']))).toContain('/ordenes/retiro');
    expect(kids(u('jefe_operaciones'))).not.toContain('/ordenes/retiro');
    expect(kids(u('empaque',['despacho']))).toEqual(['/ordenes/empaque','/ordenes/retiro']);
    expect(kids(u('admin'))).toContain('/ordenes/retiro');
    expect(kids(u('cliente',['despacho']))).not.toContain('/ordenes/retiro');
  });
  it('la ruta exige el permiso, aunque /ordenes sea del Cliente',()=>{
    expect(canAccessPath(u('empaque',['despacho']),'/ordenes/retiro')).toBe(true);
    expect(canAccessPath(u('empaque'),'/ordenes/retiro')).toBe(false);
    expect(canAccessPath(u('cliente'),'/ordenes/retiro')).toBe(false);
    expect(canAccessPath(u('admin'),'/ordenes/retiro')).toBe(true);
  });
  it('la ruta antigua ya no existe como pantalla',()=>{
    expect(visibleMenu(u('admin')).flatMap(e=>e.children??[]).some(c=>c.href==='/retiro')).toBe(false);
  });
});
