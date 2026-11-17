import { describe, it, expect } from 'vitest';
import { canAccessPath, visibleMenu } from '@/state/menuAccess';
import { canManageProfiles } from '@/features/clients/access';

// Tarea 07 · Fase 1: hoja «Menú por perfil» v1.2 de la Matriz de perfiles.
const u = (role, permissions = []) => ({ role, permissions });
const clients = (user) => visibleMenu(user).find((e) => e.name === 'Clientes')?.children?.map((c) => c.href);
const entry = (user, name) => visibleMenu(user).find((e) => e.name === name);

describe('Clientes en el menú', () => {
  it('Perfiles empresa solo para el Cliente; Administración clientes para el Administrador', () => {
    expect(clients(u('cliente'))).toEqual(['/clientes/perfiles-empresa']);
    expect(clients(u('admin'))).toEqual(['/clientes/administracion']);
    expect(clients(u('admin', ['gestionar_perfiles']))).toEqual(['/clientes/administracion']);
  });
  it('el Jefe de Operaciones, Empaque y Analista ya no ven Clientes', () => {
    for (const r of ['jefe_operaciones', 'empaque', 'analista']) expect(entry(u(r, ['despacho']), 'Clientes')).toBeUndefined();
  });
  it('se valida por permiso: otro perfil con «Gestionar perfiles» entra a Administración clientes', () => {
    expect(clients(u('jefe_operaciones', ['gestionar_perfiles']))).toEqual(['/clientes/administracion']);
    expect(canAccessPath(u('jefe_operaciones', ['gestionar_perfiles']), '/clientes/administracion')).toBe(true);
  });
});

describe('Clientes por URL', () => {
  it('cada submenú exige su perfil o el permiso', () => {
    expect(canAccessPath(u('cliente'), '/clientes/perfiles-empresa')).toBe(true);
    expect(canAccessPath(u('cliente'), '/clientes/administracion')).toBe(false);
    expect(canAccessPath(u('admin'), '/clientes/administracion')).toBe(true);
    expect(canAccessPath(u('admin'), '/clientes/perfiles-empresa')).toBe(false);
    expect(canAccessPath(u('jefe_operaciones', ['despacho']), '/clientes')).toBe(false);
    expect(canAccessPath(u('jefe_operaciones'), '/clientes/administracion')).toBe(false);
  });
});

describe('Integraciones', () => {
  it('el Administrador lo ve como enlace a la página de relleno; el resto no', () => {
    expect(entry(u('admin'), 'Integraciones')).toMatchObject({ href: '/integraciones', disabled: false });
    expect(canAccessPath(u('admin'), '/integraciones')).toBe(true);
    for (const r of ['jefe_operaciones', 'empaque', 'cliente']) {
      expect(entry(u(r), 'Integraciones')).toBeUndefined();
      expect(canAccessPath(u(r), '/integraciones')).toBe(false);
    }
  });
});

describe('canManageProfiles', () => {
  it('pregunta por el permiso, nunca por el perfil', () => {
    expect(canManageProfiles(u('admin'), 'client')).toBe(false);
    expect(canManageProfiles(u('admin', ['gestionar_perfiles']), 'client')).toBe(true);
    expect(canManageProfiles(u('admin', ['gestionar_perfiles']), 'internal')).toBe(true);
    expect(canManageProfiles(u('admin', ['despacho']), 'client')).toBe(false);
    expect(canManageProfiles(null, 'client')).toBe(false);
  });
});
