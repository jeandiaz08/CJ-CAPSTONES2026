-- Datos ficticios SOLO para desarrollo. Lo ejecuta el servicio seed-dev del override.
-- Se puede correr varias veces: no pisa lo existente, pero recrea lo que se haya borrado.
-- Los usuarios de prueba y su contraseña están en infrastructure/keycloak/dev-users.
BEGIN;

INSERT INTO companies (id, name, tax_id, wms_code) VALUES
  (1, 'Empresa Demo Uno SpA', '11.111.111-1', 'DEMO1'),
  (2, 'Empresa Demo Dos Ltda', '22.222.222-2', 'DEMO2')
ON CONFLICT DO NOTHING;
SELECT setval('companies_id_seq', GREATEST((SELECT MAX(id) FROM companies), 1));

INSERT INTO users (id, email, name, role, permissions, all_companies) VALUES
  (1, 'admin@nexus.local',    'Admin Demo',               'admin',            '{}',         TRUE),
  (2, 'jefe@nexus.local',     'Jefe de Operaciones Demo', 'jefe_operaciones', '{despacho}', TRUE),
  (3, 'empaque@nexus.local',  'Empaque Demo',             'empaque',          '{despacho}', TRUE),
  (4, 'analista@nexus.local', 'Analista Demo',            'analista',         '{}',         TRUE),
  (5, 'cliente1@nexus.local', 'Cliente Demo Uno',         'cliente',          '{}',         FALSE)
ON CONFLICT DO NOTHING;
SELECT setval('users_id_seq', GREATEST((SELECT MAX(id) FROM users), 1));
INSERT INTO users (email, name, role, permissions, all_companies)
VALUES ('cliente2@nexus.local', 'Cliente Demo Dos', 'cliente', '{}', FALSE)
ON CONFLICT DO NOTHING;

INSERT INTO user_companies (user_id, company_id)
SELECT u.id, c.id FROM users u JOIN companies c ON
  (u.email = 'cliente1@nexus.local' AND c.tax_id = '11.111.111-1') OR
  (u.email = 'cliente2@nexus.local' AND c.tax_id = '22.222.222-2')
ON CONFLICT DO NOTHING;

INSERT INTO products (company_id, sku, description)
SELECT c.id, v.sku, v.description FROM companies c JOIN (VALUES
  ('11.111.111-1', 'SKU-001', 'Caja mediana demo'),
  ('11.111.111-1', 'SKU-002', 'Polera talla M demo'),
  ('11.111.111-1', 'SKU-SINSTOCK', 'Producto sin stock (para probar Anulada)'),
  ('22.222.222-2', 'SKU-900', 'Producto de otra empresa')
) AS v(tax_id, sku, description) ON v.tax_id = c.tax_id
ON CONFLICT DO NOTHING;

COMMIT;
