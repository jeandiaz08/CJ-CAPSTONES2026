-- ============================================================================
-- DATOS DE PRUEBA (ficticios). ELIMINAR o reemplazar antes de producción.
-- Contraseña de TODOS los usuarios de prueba: Nexus2026!
-- (hash generado con:  docker compose exec core /app/tmp/api -hash-password 'Nexus2026!')
-- ============================================================================
INSERT INTO companies (id, name, tax_id, wms_code) VALUES
  (1, 'Empresa Demo Uno SpA', '11.111.111-1', 'DEMO1'),
  (2, 'Empresa Demo Dos Ltda', '22.222.222-2', 'DEMO2')
ON CONFLICT (id) DO NOTHING;
SELECT setval('companies_id_seq', GREATEST((SELECT MAX(id) FROM companies), 1));

INSERT INTO users (id, email, name, password_hash, role, permissions, all_companies) VALUES
  (1, 'admin@nexus.local',    'Admin Demo',              'pbkdf2-sha256$210000$qDzcGjBS/wAw9fOk9HlzoQ$7BFXTnyNYXdNiY1bJ/ftiaW4BSu6mP7rJLi0/b9tpZ0', 'admin',            '{}',         TRUE),
  (2, 'jefe@nexus.local',     'Jefe de Operaciones Demo','pbkdf2-sha256$210000$qDzcGjBS/wAw9fOk9HlzoQ$7BFXTnyNYXdNiY1bJ/ftiaW4BSu6mP7rJLi0/b9tpZ0', 'jefe_operaciones', '{despacho}', TRUE),
  (3, 'empaque@nexus.local',  'Empaque Demo',            'pbkdf2-sha256$210000$qDzcGjBS/wAw9fOk9HlzoQ$7BFXTnyNYXdNiY1bJ/ftiaW4BSu6mP7rJLi0/b9tpZ0', 'empaque',          '{despacho}', TRUE),
  (4, 'analista@nexus.local', 'Analista Demo',           'pbkdf2-sha256$210000$qDzcGjBS/wAw9fOk9HlzoQ$7BFXTnyNYXdNiY1bJ/ftiaW4BSu6mP7rJLi0/b9tpZ0', 'analista',         '{}',         TRUE),
  (5, 'cliente1@nexus.local', 'Cliente Demo Uno',        'pbkdf2-sha256$210000$qDzcGjBS/wAw9fOk9HlzoQ$7BFXTnyNYXdNiY1bJ/ftiaW4BSu6mP7rJLi0/b9tpZ0', 'cliente',          '{}',         FALSE)
ON CONFLICT (id) DO NOTHING;
SELECT setval('users_id_seq', GREATEST((SELECT MAX(id) FROM users), 1));
INSERT INTO user_companies (user_id, company_id) VALUES (5, 1) ON CONFLICT DO NOTHING;

INSERT INTO products (company_id, sku, description) VALUES
  (1, 'SKU-001', 'Caja mediana demo'),
  (1, 'SKU-002', 'Polera talla M demo'),
  (1, 'SKU-SINSTOCK', 'Producto sin stock (para probar Anulada)'),
  (2, 'SKU-900', 'Producto de otra empresa')
ON CONFLICT (company_id, sku) DO NOTHING;

INSERT INTO regions (code, name, sort_order) VALUES
  ('XV','Arica y Parinacota',1),('I','Tarapacá',2),('II','Antofagasta',3),
  ('III','Atacama',4),('IV','Coquimbo',5),('V','Valparaíso',6),
  ('RM','Metropolitana de Santiago',7),('VI','O''Higgins',8),('VII','Maule',9),
  ('XVI','Ñuble',10),('VIII','Biobío',11),('IX','La Araucanía',12),
  ('XIV','Los Ríos',13),('X','Los Lagos',14),('XI','Aysén',15),('XII','Magallanes',16)
ON CONFLICT (code) DO NOTHING;

INSERT INTO couriers (code, name) VALUES
  ('chilexpress','ChileExpress'), ('blueexpress','BlueExpress'),
  ('starken','Starken'), ('mercadoenvios','Mercado Envíos')
ON CONFLICT (code) DO NOTHING;
