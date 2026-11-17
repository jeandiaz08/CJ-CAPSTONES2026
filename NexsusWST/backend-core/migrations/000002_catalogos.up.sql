-- Catálogos reales: regiones y couriers.
-- Los datos ficticios de desarrollo están en infrastructure/dev-seed/seed_dev.sql.
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
